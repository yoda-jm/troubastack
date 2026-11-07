# win08-cost.ps1 — WIN08 S2.11, what Studio costs. Run by ci.yml's dispatch-only `win08-cost` job (see there), on the
# Windows runner, after the job has started a loopback core with the demo song and packaged the app.
#
# Measures, in this one job: TroubaStage on Home (Studio never opened); TroubaStage with the demo song open in Studio;
# the same after leaving and re-entering Studio five times; and Microsoft Edge on the same song, signed in to Studio.
# Each figure is a sum over one process tree found by parent pid (the app's launcher and everything under it; Edge's
# browser process and everything under it), taken once it has settled: 3 samples 2 s apart within 2 %, 60 s at most.
# The figure is the working set (WorkingSet64, as stage 1's 614 MiB), with private bytes (PrivateMemorySize64)
# alongside. A working-set sum counts pages shared between processes once per process; private bytes don't.
#
# Two leak checks, each the reviewer's bound:
#   - WebView2: its processes' working set at the 6th Studio entry within +10 % of the first entry's, same count;
#   - JVM: the used heap after System.gc() (the hook logs it on leaving each Home) at the Home after the 6th entry
#     within +16 MiB of the Home after the first. The JVM's working set is not a leak signal: the heap grows and
#     rarely gives memory back. WIN08_COST_SABOTAGE=leak (64 MiB kept per entry) must red this check.
param(
    [Parameter(Mandatory)] [string] $Exe,
    [Parameter(Mandatory)] [string] $Out,
    [Parameter(Mandatory)] [string] $SongPath,
    [Parameter(Mandatory)] [string] $CookieFile,
    [string] $Core = "http://127.0.0.1:8080"
)
$ErrorActionPreference = "Stop"
$MiB = 1MB
$WebKinds = @("msedgewebview2", "msedge")

function Tree([int] $rootPid) {
    $acc = @($rootPid)
    foreach ($c in @(Get-CimInstance Win32_Process -Filter "ParentProcessId=$rootPid" -ErrorAction SilentlyContinue)) { $acc += Tree ([int]$c.ProcessId) }
    $acc
}
function Total($procs, [string] $prop) { $t = [int64]0; foreach ($q in $procs) { $t += [int64]$q.$prop }; $t }
function Sample([int] $rootPid) {
    $ps = @(Get-Process -Id @(Tree $rootPid | Select-Object -Unique) -ErrorAction SilentlyContinue)
    $web = @($ps | Where-Object { $WebKinds -contains $_.ProcessName })
    $own = @($ps | Where-Object { $WebKinds -notcontains $_.ProcessName })
    [pscustomobject]@{
        ws = Total $ps "WorkingSet64"; priv = Total $ps "PrivateMemorySize64"; n = $ps.Count
        webWs = Total $web "WorkingSet64"; webPriv = Total $web "PrivateMemorySize64"; webN = $web.Count
        ownWs = Total $own "WorkingSet64"; ownN = $own.Count
        names = (($ps | Group-Object ProcessName | ForEach-Object { "$($_.Name)x$($_.Count)" }) -join ",")
    }
}
function Settle([int] $rootPid, [string] $label) {
    $t0 = Get-Date; $hist = New-Object System.Collections.Generic.List[double]; $settled = $false
    while ($true) {
        $s = Sample $rootPid
        $hist.Add([double]$s.ws)
        if ($hist.Count -ge 3) {
            $last = $hist.GetRange($hist.Count - 3, 3)
            $mx = ($last | Measure-Object -Maximum).Maximum; $mn = ($last | Measure-Object -Minimum).Minimum
            if ($mn -gt 0 -and $mx -le 1.02 * $mn) { $settled = $true; break }
        }
        if (((Get-Date) - $t0).TotalSeconds -ge 60) { break }
        Start-Sleep -Seconds 2
    }
    $secs = [int]((Get-Date) - $t0).TotalSeconds
    $s | Add-Member -NotePropertyName label -NotePropertyValue $label
    $s | Add-Member -NotePropertyName settled -NotePropertyValue $settled
    Write-Host ("WIN08-COST {0,-9} ws {1,7:N1} MiB  private {2,7:N1} MiB  {3,2} procs | web {4,7:N1} MiB in {5} | own {6,6:N1} MiB | settled={7} in {8}s [{9}]" -f `
        $label, ($s.ws / $MiB), ($s.priv / $MiB), $s.n, ($s.webWs / $MiB), $s.webN, ($s.ownWs / $MiB), $settled, $secs, $s.names)
    $s
}
# One CDP call on its own connection: send, then read until the answer with our id.
function Cdp([string] $wsUrl, [string] $method, [hashtable] $params) {
    $ws = [System.Net.WebSockets.ClientWebSocket]::new(); $ct = [System.Threading.CancellationToken]::None
    try {
        $ws.ConnectAsync([Uri]$wsUrl, $ct).GetAwaiter().GetResult()
        $id = Get-Random -Minimum 1 -Maximum 1000000
        $bytes = [Text.Encoding]::UTF8.GetBytes((@{ id = $id; method = $method; params = $params } | ConvertTo-Json -Depth 10 -Compress))
        $ws.SendAsync([ArraySegment[byte]]::new($bytes), [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $ct).GetAwaiter().GetResult()
        $buf = [byte[]]::new(1MB)
        for ($k = 0; $k -lt 200; $k++) {
            $ms = [IO.MemoryStream]::new()
            do { $r = $ws.ReceiveAsync([ArraySegment[byte]]::new($buf), $ct).GetAwaiter().GetResult(); $ms.Write($buf, 0, $r.Count) } while (-not $r.EndOfMessage)
            $msg = [Text.Encoding]::UTF8.GetString($ms.ToArray()) | ConvertFrom-Json
            if ($msg.id -eq $id) { if ($msg.error) { throw "CDP ${method}: $($msg.error.message)" }; return $msg.result }
        }
        throw "CDP ${method}: no answer"
    } finally { $ws.Dispose() }
}
function StopTree([int] $rootPid) { foreach ($q in @(Tree $rootPid | Select-Object -Unique)) { Stop-Process -Id $q -Force -ErrorAction SilentlyContinue } }

# ---- TroubaStage: Home, Studio, and five leaves and re-entries, in one process (the --win08-cost hook) ----
New-Item -ItemType Directory -Force $Out | Out-Null
Remove-Item (Join-Path $Out "next"), (Join-Path $Out "phase") -ErrorAction SilentlyContinue
$phaseFile = Join-Path $Out "phase"
$p = Start-Process -FilePath $Exe -ArgumentList @("--win08-cost", $Out, $Core) -PassThru
function Win08Log { Get-ChildItem -Recurse -Filter win08.log $env:TROUBASTAGE_HOME -ErrorAction SilentlyContinue | Select-Object -First 1 }
function Readies { $f = Win08Log; if ($f) { @(Select-String -Path $f.FullName -Pattern "win08: ready \(").Count } else { 0 } }
function WaitFor([scriptblock] $cond, [int] $sec, [string] $what) {
    $end = (Get-Date).AddSeconds($sec)
    while ((Get-Date) -lt $end) {
        if (& $cond) { return }
        if ($p.HasExited) { throw "the app exited (code $($p.ExitCode)) while waiting for $what" }
        Start-Sleep -Milliseconds 250
    }
    throw "timed out after ${sec}s waiting for $what"
}
$phases = @("home-0") + @(1..6 | ForEach-Object { "studio-$_"; "home-$_" })
$r = [ordered]@{}
$readiesBefore = 0
try {
    foreach ($name in $phases) {
        WaitFor { (Test-Path $phaseFile) -and ((Get-Content $phaseFile -Raw) -match " $([regex]::Escape($name))\s*$") } 180 "phase $name"
        if ($name -like "studio-*") { WaitFor { (Readies) -gt $readiesBefore } 120 "Studio's ready handshake ($name)" }
        else { Start-Sleep -Seconds 3 }
        $r[$name] = Settle $p.Id $name
        if ($name -like "home-*") { $readiesBefore = Readies }
        New-Item -ItemType File (Join-Path $Out "next") | Out-Null
    }
    $end = (Get-Date).AddSeconds(60)
    while (-not $p.HasExited -and (Get-Date) -lt $end) { Start-Sleep -Milliseconds 250 }
    if (-not $p.HasExited) { throw "the app did not exit after its last phase" }
} finally {
    Write-Host "---- cost.log ----"; Get-Content (Join-Path $Out "cost.log") -ErrorAction SilentlyContinue | Write-Host
    if (-not $p.HasExited) { StopTree $p.Id }
}
$heap = @{}
foreach ($l in Get-Content (Join-Path $Out "cost.log")) { if ($l -match "WIN08-COST heap (home-\d) used=(\d+)") { $heap[$Matches[1]] = [int64]$Matches[2] } }
$missing = @(0..6 | Where-Object { -not $heap.ContainsKey("home-$_") })
if ($missing.Count -gt 0) { throw "cost.log has no used-heap line for home-$($missing -join ', home-')" }

# ---- Microsoft Edge on the same song, signed in to Studio (its session cookie, set through CDP) ----
$edgeExe = @("${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe", "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $edgeExe) { throw "Microsoft Edge is not installed on this runner" }
$prof = Join-Path $env:RUNNER_TEMP "win08-cost-edge"
$port = 9333
$null = Start-Process -FilePath $edgeExe -ArgumentList @("--user-data-dir=$prof", "--no-first-run", "--no-default-browser-check", "--remote-debugging-port=$port", "--remote-allow-origins=*", "--window-position=0,0", "--window-size=1400,900", "about:blank") -PassThru
$ver = $null; $end = (Get-Date).AddSeconds(30)
while (-not $ver -and (Get-Date) -lt $end) { try { $ver = Invoke-RestMethod "http://127.0.0.1:$port/json/version" } catch { Start-Sleep -Milliseconds 300 } }
if (-not $ver) { throw "Edge's debugging port never answered" }
# DevTools names its WebSocket host after the request; pin it to the loopback address the port listens on.
function Local([string] $u) { $u -replace '^ws://[^/]+', "ws://127.0.0.1:$port" }
# Invoke-RestMethod hands a JSON array down the pipe as ONE object: assign it first, so piping the variable enumerates.
function Pages { $list = Invoke-RestMethod "http://127.0.0.1:$port/json/list"; @($list | Where-Object { $_.type -eq "page" }) }
# The browser process is the msedge.exe on this profile without --type= (renderer, GPU and utility processes carry one).
$browser = @(Get-CimInstance Win32_Process -Filter "Name='msedge.exe'" | Where-Object { $_.CommandLine -like "*$prof*" -and $_.CommandLine -notlike "*--type=*" }) | Select-Object -First 1
if (-not $browser) { throw "no Edge browser process on the profile $prof" }
try {
    $cookie = (Get-Content $CookieFile -Raw).Trim()
    $null = Cdp (Local $ver.webSocketDebuggerUrl) "Storage.setCookies" @{ cookies = @(@{ name = "trouba_session"; value = $cookie; url = $Core; path = "/"; httpOnly = $true; sameSite = "Lax" }) }
    $pages = Pages
    $page = $pages[0]; $pageWs = Local $page.webSocketDebuggerUrl
    foreach ($extra in @($pages | Select-Object -Skip 1)) { $null = Invoke-RestMethod "http://127.0.0.1:$port/json/close/$($extra.id)" }
    $null = Cdp $pageWs "Page.navigate" @{ url = "$Core$SongPath" }
    $state = ""; $end = (Get-Date).AddSeconds(60)
    while ($state -ne "complete" -and (Get-Date) -lt $end) {
        Start-Sleep -Seconds 1
        $state = (Cdp $pageWs "Runtime.evaluate" @{ expression = "document.readyState"; returnByValue = $true }).result.value
    }
    Start-Sleep -Seconds 3
    $seen = (Cdp $pageWs "Runtime.evaluate" @{ expression = "fetch('/api/me').then(x => x.status + ' ' + location.pathname)"; awaitPromise = $true; returnByValue = $true }).result.value
    $tabs = (Pages).Count
    Write-Host "Edge: readyState=$state; /api/me and the path: '$seen'; $tabs tab(s); flags: a fresh profile, --no-first-run, --no-default-browser-check, --remote-debugging-port, --remote-allow-origins"
    if ($seen -notlike "200 $SongPath*") { throw "Edge is not signed in on the song's page (expected '200 $SongPath…', got '$seen')" }
    $r["edge"] = Settle ([int]$browser.ProcessId) "edge"
} finally { StopTree ([int]$browser.ProcessId) }

# ---- The figures and the two leak checks ----
function M($b) { "{0:N1} MiB" -f ($b / $MiB) }
$first = $r["studio-1"]; $sixth = $r["studio-6"]
Write-Host ""
Write-Host "==== WIN08 S2.11 - what it costs: the sum over each process tree, settled ===="
foreach ($row in @(@("TroubaStage on Home, Studio never opened", "home-0"), @("TroubaStage with the demo song open in Studio", "studio-1"),
        @("Microsoft Edge on the same song, signed in", "edge"), @("TroubaStage after 5 leaves and re-entries", "studio-6"))) {
    $s = $r[$row[1]]
    Write-Host ("WIN08-COST FIGURE {0,-48} working set {1,10}   private {2,10}   {3,2} processes   settled={4}" -f $row[0], (M $s.ws), (M $s.priv), $s.n, $s.settled)
}
Write-Host ("WIN08-COST series (WebView2 working set by Studio entry): " + ((1..6 | ForEach-Object { "$_ " + (M $r["studio-$_"].webWs) + " in $($r["studio-$_"].webN)" }) -join " | "))
Write-Host ("WIN08-COST series (the whole tree's working set by Studio entry): " + ((1..6 | ForEach-Object { "$_ " + (M $r["studio-$_"].ws) }) -join " | "))
Write-Host ("WIN08-COST series (WebView2 processes left at each Home): " + ((0..6 | ForEach-Object { "home-$_ $($r["home-$_"].webN)" }) -join " | "))
Write-Host ("WIN08-COST series (used heap after GC, leaving each Home): " + ((0..6 | ForEach-Object { "home-$_ " + (M $heap["home-$_"]) }) -join " | "))
$noiseWeb = 100.0 * ($r["studio-2"].webWs - $first.webWs) / $first.webWs
Write-Host ("WIN08-COST noise: WebView2 first re-entry against first open {0:+0.0;-0.0} %; used heap home-2 against home-1 {1}" -f $noiseWeb, (M ($heap["home-2"] - $heap["home-1"])))
$fails = 0
$webOk = ($sixth.webWs -le 1.10 * $first.webWs) -and ($sixth.webN -eq $first.webN)
Write-Host ("WIN08-COST CHECK webview2-comes-back: {0} - 6th entry {1} in {2} against the first {3} in {4} (bound +10 %, same count)" -f ($(if ($webOk) { "PASS" } else { "FAIL" })), (M $sixth.webWs), $sixth.webN, (M $first.webWs), $first.webN)
if (-not $webOk) { $fails++ }
$jvmOk = ($heap["home-6"] - $heap["home-1"]) -le 16MB
Write-Host ("WIN08-COST CHECK jvm-heap-comes-back: {0} - used heap after GC at home-6 {1} against home-1 {2} (bound +16 MiB)" -f ($(if ($jvmOk) { "PASS" } else { "FAIL" })), (M $heap["home-6"]), (M $heap["home-1"]))
if (-not $jvmOk) { $fails++ }
if ($fails -gt 0) { Write-Error "$fails WIN08-COST check(s) failed"; exit 1 }
