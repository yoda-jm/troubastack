package com.troubastack.shared.stage

/**
 * A77 §7 — the "whether" of the armed auto-send, kept in shared and PURE so it is tested off-device. Fable's
 * line: the host owns only WHEN (the debounce and the flush triggers); anything deciding whether a send is
 * allowed — or how to read its result — lives here. The host maps its transport outcome to [SendResultKind]
 * and tracks which note keys it has already auto-sent THIS armed session; these functions decide the rest.
 */

/** The transport outcome reduced to what the armed policy cares about (the host maps its NoteSendResult here). */
enum class SendResultKind { OK, CONFLICT, FAILED }

/** The verdict for one armed auto-send attempt. */
enum class ArmedSendVerdict { SENT, CONFLICT_DISARM, FAILED }

/**
 * Whether an auto-send carries `overwrite`. The FIRST auto-send of a note in this session goes WITHOUT it, so
 * a note already on the server that we did NOT put (a manual send, or another member's) returns 409 instead
 * of being clobbered. Once we have auto-sent this note, later sends replace OUR OWN prior send, so they
 * overwrite. This is the exact distinction §7 draws against the bulk path's unconditional overwrite.
 */
fun armedSendOverwrite(alreadyAutoSent: Boolean): Boolean = alreadyAutoSent

/**
 * The verdict for an attempt's [result]. A CONFLICT can only come back on a first (non-overwrite) send — it
 * means a note we did not put is there, so DO NOT clobber: disarm and report. OK ⇒ the caller records the key
 * as sent this session (so its next send overwrites). FAILED ⇒ report, but stay armed — a transient network
 * error is not a conflict, and disarming on it would punish the offline case A77 is built around.
 */
fun armedSendVerdict(result: SendResultKind): ArmedSendVerdict = when (result) {
    SendResultKind.OK -> ArmedSendVerdict.SENT
    SendResultKind.CONFLICT -> ArmedSendVerdict.CONFLICT_DISARM
    SendResultKind.FAILED -> ArmedSendVerdict.FAILED
}
