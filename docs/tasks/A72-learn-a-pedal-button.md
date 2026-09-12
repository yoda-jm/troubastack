# A72 — Learn a pedal button (Parameters)

**Lane:** mobile · **Status:** specced, not started · **Origin:** VLL, 2026-09-12 — a 4-button Bluetooth
foot pedal; *"ca ne marchait pas, aucun des 4 bouton, dans les preference un bouton learn serait top pour
apprendre precedent et suivant?"*

## 1. Why none of the four buttons did anything

`stageKeyAction` (`app/shared/.../stage/StageKeys.kt`) is a **fixed map of nine keys** — PageUp/PageDown,
the four arrows, Space, and the two volume keys. Those are the codes a *two-pedal* page-turner sends. A
four-button unit is usually configurable and commonly ships sending something else entirely: letters, F-keys,
or media transport codes. Four buttons doing nothing is the exact signature of four codes outside that map.

The capture itself is **not** suspect and was checked before filing: `StageScreen.kt:589-601` attaches
`focusRequester(keyFocus).focusable().onPreviewKeyEvent { … }`, and `:397` requests focus when
`holdsKeyFocus`. Keyboard input does reach the Stage. The Android volume keys additionally come through
`MainActivity.onKeyDown`.

**The alternative cause this task must be able to distinguish:** the pedal may be a **BLE-MIDI** device, not
an HID keyboard. There is **no MIDI path anywhere in the app** (verified: no match for `midi` under `app/`).
If it speaks MIDI, no amount of learning helps — a learn mode binds an event it *receives*; it cannot
conjure a transport. That is a separate and much larger task (Android `MidiManager` + BLE MIDI, and its
CoreMIDI counterpart on iOS), and it is explicitly **out of scope here**.

## 2. Scope

A **Learn** control in the Parameters screen (`app/shared/.../ui/SettingsScreen.kt`) that binds an arbitrary
key to **Next page** and to **Previous page**. Two actions. Nothing else.

## 3. Decisions

### ⟨D1⟩ The learn panel is the diagnostic, and must distinguish "nothing" from "unknown"

This is the requirement that earns the task its keep. While learning is armed the panel shows, live:

- **nothing received yet** — an explicit resting state, not an empty box that could equally mean "broken";
- **received: `<raw code>`** — shown for **every** key press, including codes the app does not recognise and
  including a press that is about to be refused by ⟨D4⟩.

Never silently swallow a press. If VLL arms Learn, presses all four pedals and the panel stays on *nothing
received yet*, **that is the task's answer, not its failure**: the pedal is not an HID keyboard, A72 stops
there, and the MIDI question gets its own task with that evidence attached. Report it as a finding.

### ⟨D2⟩ Learned bindings are added to the defaults; they never replace them

The eight built-in keys keep working exactly as today. Reason: a two-pedal unit that works now must not break
because someone taught the app a new button. Provide **Forget learned buttons**, which clears the learned map
only and never touches the defaults.

### ⟨D3⟩ One key, one action — last learned wins, visibly

If a key already bound to Previous is learned as Next, the old binding is **removed**, not shadowed. Leaving
one code on two actions would let evaluation order decide behaviour invisibly. Say so on screen when it
happens ("this button was Previous").

### ⟨D4⟩ Back and Home can never be learned

Binding Back traps the user inside the Stage with no way out; Home is the OS's. Refuse both — and per ⟨D1⟩
still *show* the press, with the reason. Volume keys stay learnable: the app already claims them.

### ⟨D5⟩ Identity is the platform key code, and the map is device-local

Persist the native code as a number, through the existing convention — hoisted values, host owns storage,
`storage.putSecret("stage.pedalBindings", …)` alongside `stage.fitMode`. A code learned on Android is
meaningless on iOS: this map **does not sync and does not travel with the account**. It describes a piece of
hardware sitting in front of one device.

### ⟨D6⟩ Keep the map a pure function

Extend to `stageKeyAction(key, learned)` with `learned` defaulting to empty, so `StageKeysTest` keeps passing
unchanged and the new precedence/conflict/refusal cases stay pure and off-device.

**And then do not mistake that for the proof.** A seam test proves the seam, never the surface — it cannot
know whether a real pedal emits anything at all, which is the entire question here. **Acceptance for A72 is
VLL's own pedal turning a page**, or ⟨D1⟩'s finding that it emits nothing. Green tests alone do not close it.

### ⟨D7⟩ Two actions now; a shape that will take more later

Store as action → codes so a third action needs no migration. Buttons 3 and 4 stay **deliberately
unassigned** — VLL has not said what they should do, and inventing "next song" or "jump to segno" now would
be guessing at his foot. That is a question for him once the first two work.

## 4. Notes for the implementer

- The Parameters screen needs its **own** focusable capture while learning is armed; it cannot borrow
  Stage's, which only lives on the Stage Box and only handles `KeyDown`.
- A pedal sending **media transport** codes may have them consumed by the OS or another app before Compose
  sees them. If ⟨D1⟩ shows nothing for some buttons but codes for others, that asymmetry is the tell — record
  it rather than concluding the pedal is mute.
