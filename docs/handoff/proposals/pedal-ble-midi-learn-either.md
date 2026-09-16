# Proposal — Foot pedal: BLE-MIDI input + learn EITHER a keyboard key or a MIDI message (absorbs A72)

**Lane:** mobile (`app/androidApp` + `app/shared`). **Raised by:** VLL, 2026-09-16/17. **Status:** proposal
for Fable to validate + spec. **Absorbs A72** — do not land A72 alone; its Learn panel changes here.

## A72 ⟨D1⟩ is resolved on real hardware: the pedal is MIDI, not an HID keyboard

VLL's pedal is an **M-VAVA "FootCtrlPlus"** — a **BLE-MIDI foot controller** (4 footswitches + three
7-segment displays; AB/CD step MIDI registers/channels, like page up/down over MIDI). It advertises HID
collections (so it pairs/connects), but its switch presses do **not** arrive as HID key reports. Measured:

- **Silent to HID at the kernel layer on two independent hosts** — Android `adb getevent` (modes U and H,
  freshly paired) and a Linux desktop's **root `libinput`** (which registered other BT devices fine).
  Battery 82%, so not power. So it is not the app, not the tablet's stack, and not a keyboard.
- **Its GATT map (read via desktop `bluetoothctl`) shows the real channel:** standard **BLE-MIDI** service
  `03b80e5a-ede8-4b33-a751-6ce34ec4c700` / char `7772e5db-3868-4112-a1a9-f2669d106bf3`, plus MVAVE's custom
  `0xae40`(`ae42` notify/`ae41` write) and `0xae00`(`ae02`). The MIDI flows over the BLE-MIDI characteristic.

VLL's original instinct ("maybe it is midi") was right; I wrongly ruled MIDI out when I first saw the HID
collections. Corrected here.

## What VLL asked for

> *"can it also work with [midi]? learn either midi or keyboard in bt?"* — and chose **"Build BLE-MIDI
> (learn either)."**

## Rough shape (for Fable to spec)

- **Keep A72's keyboard-key learning** unchanged (`stageKeyAction(key, learned)`, `PedalBindings`, refuse
  rules, persisted `stage.pedalBindings`, tests) — correct for HID pedals.
- **Add a BLE-MIDI input path (Android):** `android.media.midi.MidiManager.openBluetoothDevice` on the
  BLE-MIDI service, subscribe, receive parsed MIDI (`MidiReceiver`), map a switch's message
  (Program-Change / Control-Change / Note) → `PageTurn`. Requires `BLUETOOTH_SCAN` + `BLUETOOTH_CONNECT`
  (Android 12+). Let the OS own the BLE link — do NOT hand-roll GATT (see the flake note below).
- **Learn either:** the armed Learn panel listens for a keyboard key **and** a MIDI message, learns whichever
  arrives; a binding is tagged by kind (`KEY:<code>` / `MIDI:<status,data1>`); the on-stage matcher turns a
  learned event of either kind into a `PageTurn`. Last-learned-wins per kind.
- **The panel stays the ⟨D1⟩ diagnostic** — show the raw thing received (a key code, or "MIDI: PC 3"), so a
  truly silent pedal still reads "nothing received yet".

## Constraints / traps

- **Connection lifecycle.** The pedal sleeps and drops the BLE link aggressively — under a generic central
  (`bluetoothctl`) it disconnected within ~2 s, three times, before a press registered. `MidiManager` should
  hold it far better (it is the purpose-built BLE-MIDI client), but the spec must cover reconnect / re-open
  on drop, and a pedal that sleeps between songs.
- **Pure-testable seams:** "MIDI message → PageTurn?" and "learn-either → tagged binding" as pure functions
  with tests (the A72/A75 pattern), separate from the BLE plumbing.
- **iOS out of scope** for the first slice (CoreMIDI is a separate `actual`); keep the port a no-op there,
  land Android-first.
- The MVAVE custom services (`0xae40`/`0xae00`) are undocumented and unnecessary — **use standard BLE-MIDI**,
  not the vendor protocol.

## Open questions for Fable

- Scan/connect UX: auto-connect the known BLE-MIDI device when the panel is armed, or an explicit "Connect
  pedal" step?
- Binding storage: extend `stage.pedalBindings` to carry kind, or a second key?
- Does a switch send a stable per-switch message, or does AB/CD register-stepping change what each switch
  emits? (The learn step sidesteps this — it captures whatever the switch sends in its current register —
  but the spec should note the register interaction.)
