// A52 — the pure, testable core of the join flow: the pending-token holder (a bearer credential, so its
// whole contract is "in memory, cleared on every terminal outcome"), and the two response mappers that
// turn a server status + body into an outcome USING THE SERVER'S OWN WORDS (a 410's reason), never a
// fabricated generic failure. The Android sheet (JoinDialog) is a thin driver over these; iOS/web can
// reuse them unchanged. See [parseTroubaLink]/[joinDecision] in JoinLink.kt for the routing that precedes
// this.
package com.troubashare.shared.join

/**
 * The pending invite token while a join flow is in progress. It is a **bearer credential**: this holder
 * keeps it in memory only (never hand it to Storage beside the session cookie — a token that outlives the
 * flow is one that can be replayed off a stolen device) and [clear]s it on success, failure, cancel, or
 * navigating away. Not thread-safe by design: it is touched only from the UI flow that owns it.
 */
class PendingToken {
    var value: String? = null
        private set

    /** Arm the flow with [token]. Overwrites any previous one (a new link supersedes an abandoned flow). */
    fun arm(token: String) { value = token }

    /** Drop the token. Idempotent — safe to call on every terminal path without checking. */
    fun clear() { value = null }

    val isArmed: Boolean get() = value != null
}

/** The outcome of `POST /api/invite-links/{token}/accept`, in the server's own words. */
sealed interface AcceptOutcome {
    /** 200 — joined; [band] is the server-named band (falls back to a neutral label if blank). */
    data class Joined(val band: String) : AcceptOutcome

    /** 410 Gone — expired / revoked / exhausted; [reason] is the server's machine-readable message. */
    data class Gone(val reason: String) : AcceptOutcome

    /** 404 — the token doesn't resolve to a link (mistyped/garbage). */
    data object NotFound : AcceptOutcome

    /** 401 — the session lapsed mid-flow; the caller must sign in again (the token survives). */
    data object NeedsSignIn : AcceptOutcome

    /** Anything else (5xx / unexpected). [status] is carried so the message can be specific, not "oops". */
    data class Failed(val status: Int) : AcceptOutcome
}

/**
 * Map an accept response to an [AcceptOutcome]. [bandName]/[reason] come straight from the body; this
 * function's only job is to route by [status] and preserve the server's words — a 410 shows [reason]
 * verbatim (the server already distinguishes expired vs revoked vs exhausted), never a generic failure.
 */
fun acceptOutcome(status: Int, bandName: String?, reason: String?): AcceptOutcome = when (status) {
    200 -> AcceptOutcome.Joined(bandName?.takeIf { it.isNotBlank() } ?: "your band")
    410 -> AcceptOutcome.Gone(reason?.takeIf { it.isNotBlank() } ?: "This invite is no longer usable.")
    404 -> AcceptOutcome.NotFound
    401 -> AcceptOutcome.NeedsSignIn
    else -> AcceptOutcome.Failed(status)
}

/**
 * The identity of a server, probed BEFORE any password is offered — `GET /api/version` is unauthenticated
 * (T123), so the join flow can verify a host the moment a scanned/deep-linked invite names it. This is the
 * REAL protection A53's scanner leans on: a hostname shown to someone who just pointed a tablet at a
 * sticker is weak; refusing the password field for a host that doesn't identify as TroubaStack is strong.
 */
sealed interface ServerIdentity {
    /** `product == "troubastack"` and its API contract is one this client understands — safe to sign in. */
    data object TroubaStack : ServerIdentity

    /** A TroubaStack server speaking a newer `/api` contract than this client knows — refuse, update first. */
    data class TooNew(val serverApi: Int, val clientMax: Int) : ServerIdentity

    /** Answered, but not a TroubaStack server (missing/wrong product marker) — refuse the password. */
    data object Foreign : ServerIdentity

    /** Couldn't probe (non-200 / network / non-JSON) — refuse rather than guess. */
    data object Unreachable : ServerIdentity
}

/** The newest `/api` contract version this client understands. Bumped when the app learns a new shape;
 *  a server advertising a higher [apiVersion] than this is refused (a client can't safely guess a contract
 *  it doesn't know). Matches the server's `apiVersion` const (currently 1). */
const val CLIENT_MAX_API_VERSION: Int = 1

/**
 * Decide a server's [ServerIdentity] from a `GET /api/version` probe. Only a 200 that names
 * `product == "troubastack"` with a known-or-older [apiVersion] is [TroubaStack]; everything else refuses.
 * A missing/blank product on a 200 is [Foreign] (a real server always stamps it), NOT a pass — the whole
 * point is that silence isn't trust.
 */
fun serverIdentity(
    status: Int,
    product: String?,
    apiVersion: Int?,
    clientMax: Int = CLIENT_MAX_API_VERSION,
): ServerIdentity = when {
    status != 200 -> ServerIdentity.Unreachable
    product != "troubastack" -> ServerIdentity.Foreign
    apiVersion == null -> ServerIdentity.Foreign
    apiVersion > clientMax -> ServerIdentity.TooNew(apiVersion, clientMax)
    else -> ServerIdentity.TroubaStack
}

/** The outcome of `GET /api/invite-links/{token}` (the preview shown before the person commits). */
sealed interface PreviewResult {
    /** A usable link — show [band] and [role] before accepting. */
    data class Ready(val band: String, val role: String) : PreviewResult

    /** Found but not usable (200 with `valid=false`): expired / revoked / exhausted — [reason] is the
     *  server's. Distinct from a network failure so the sheet can say WHY, not just "couldn't load". */
    data class Unusable(val reason: String) : PreviewResult

    /** 401 — not signed in to this server yet; sign in, then preview. */
    data object NeedsSignIn : PreviewResult

    /** 404 — no such token. */
    data object NotFound : PreviewResult

    /** Anything else. */
    data class Failed(val status: Int) : PreviewResult
}

/**
 * Map a preview response to a [PreviewResult]. A 200 carries `valid` + optional `reason`: valid ⇒ [Ready],
 * else [Unusable] with the reason. Auth/absence route to [NeedsSignIn]/[NotFound] so the sheet advances
 * the flow (sign in) rather than showing a dead error.
 */
fun previewOutcome(status: Int, band: String?, role: String?, valid: Boolean, reason: String?): PreviewResult =
    when (status) {
        200 -> if (valid) {
            PreviewResult.Ready(band?.takeIf { it.isNotBlank() } ?: "your band", role.orEmpty())
        } else {
            PreviewResult.Unusable(reason?.takeIf { it.isNotBlank() } ?: "This invite is no longer usable.")
        }
        401 -> PreviewResult.NeedsSignIn
        404 -> PreviewResult.NotFound
        else -> PreviewResult.Failed(status)
    }

/** A57 — the outcome of `POST /api/auth/register` (creating an account from an invite). */
sealed interface RegisterOutcome {
    /** 200/201 — account created; the flow signs in automatically and continues the same join. */
    data object Created : RegisterOutcome

    /** 409 — the username is taken. The common, RECOVERABLE failure: say so and keep the person in the
     *  form so they pick another name — never flattened into a generic error. */
    data object NameTaken : RegisterOutcome

    /** Anything else (5xx / unexpected / network sentinel 0). */
    data class Failed(val status: Int) : RegisterOutcome
}

/**
 * A57 — map a register response to a [RegisterOutcome]. `409` (name taken) is the outcome that matters and
 * is kept distinct from [Failed] so the sheet can say "that name is taken" and leave the person in the form
 * rather than dead-ending them. This does NOT widen what's possible — `POST /api/auth/register` is already
 * unauthenticated and open; the app just makes the supported path reachable by a human.
 */
fun registerOutcome(status: Int): RegisterOutcome = when (status) {
    200, 201 -> RegisterOutcome.Created
    409 -> RegisterOutcome.NameTaken
    else -> RegisterOutcome.Failed(status)
}
