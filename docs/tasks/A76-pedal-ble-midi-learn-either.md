# A76 — The pedal speaks MIDI: BLE-MIDI input, and Learn takes either kind

**Lane:** mobile · **Status:** specced, not started · **Absorbs A72** (do not land A72 alone).
Validates `proposals/pedal-ble-midi-learn-either.md`, whose shape is adopted as written.

## 1. ⟨D1⟩ of A72 is answered, and the panel did its job

A72's load-bearing decision was that the Learn panel had to distinguish *"nothing received yet"* from
*"received `<raw code>`"*, so that an empty panel would be **the answer rather than a failure**. It was:

- **Silent to HID at the kernel layer on two independent hosts** — Android `getevent` and a Linux desktop's
  root `libinput`, the latter registering other Bluetooth devices fine. A positive control on both, so the
  silence means something.
- **Its GATT map names the real channel**: standard BLE-MIDI service `03b80e5a…` / characteristic
  `7772e5db…`. It advertises HID collections — which is why it pairs, and why it looked like a keyboard.

So the transport question is closed on evidence rather than estimated. **A72's keyboard learning stays** —
it is correct and tested, and it is what an HID pedal needs; it is simply not what VLL owns.

## 2. ⟨D1⟩ Connect is explicit once, automatic after — because the diagnostic depends on it

Not auto-connect on arming. Two reasons, and the second is the one that decides it:

- A BLE scan needs `BLUETOOTH_SCAN`/`BLUETOOTH_CONNECT`, and a runtime permission prompt appearing because
  someone opened Parameters is a prompt with no visible cause.
- **Without a visible connection state, "nothing received yet" stops being a diagnostic.** Silence from a
  disconnected pedal and silence from a mute one look identical, and A72's whole value was telling those
  apart. The panel must show the link: *not connected / connected / nothing received yet / received X*.

After the first successful connect, reconnect automatically on entering Stage. The pedal drops the link
aggressively (§4), so reconnection is normal operation, not an error path.

## 3. ⟨D2⟩ One binding store, entries tagged by kind

Extend `stage.pedalBindings`; do **not** add a second key. Two keys would be two lifetimes for one concept —
*"Forget learned buttons"* would become two operations that can diverge, and the next reader would have to
learn which wins. Tag each entry (`KEY:<code>` / `MIDI:<status,data1>`) and keep one store, one clear, one
migration point.

## 4. ⟨D3⟩ Measure whether a switch's message is stable BEFORE writing the matcher

The proposal notes that AB/CD register-stepping may change what a switch emits, and that the learn step
"sidesteps" it. **It sidesteps it at learn time only.** At use time the consequence is worse than not
working: VLL learns a switch, it works, he steps a register mid-set, and the binding silently stops matching.
*"It worked yesterday"* is the hardest class of bug to report and the easiest to blame on the app.

So: **press each switch in at least two registers and record what arrives.** Then spec the matcher against
what was measured:

- stable per switch → match on the full message, done;
- varies by register → the matcher must key on the stable part, or the spec tells the user plainly that a
  binding belongs to a register. **Either is acceptable; silently unbinding is not.**

This is the same shape as a proxy condition that holds at the moment you check it and fails in the middle.

## 5. Adopted from the proposal without change

- **Let the OS own the link**: `MidiManager.openBluetoothDevice`, not hand-rolled GATT. The ~2 s disconnects
  seen under a generic central are the evidence for this, not against it.
- **The vendor services (`0xae40`/`0xae00`) are out of scope** — undocumented and unnecessary.
- **Pure seams with tests**: *MIDI message → PageTurn?* and *learn-either → tagged binding*, separate from
  the BLE plumbing (the A72/A75 pattern).
- **iOS is a no-op port for this slice**; land Android first.
- **The panel remains the ⟨D1⟩ diagnostic**, now showing either kind — a key code, or `MIDI: PC 3`.

## 6. Acceptance

- **VLL's own pedal turns a page**, on his tablet. That was A72's acceptance and it is unchanged; nothing
  else closes this.
- A page turn still works from a keyboard pedal and from the built-in keys — the HID path is not regressed.
- Reconnect after the pedal sleeps, shown on the device, not argued.
- The panel distinguishes all four states of §2.
