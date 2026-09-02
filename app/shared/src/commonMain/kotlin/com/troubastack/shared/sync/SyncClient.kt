// Generated proto types (ClientMessage, ServerMessage, Mutation, Snapshot, Rejection, …) come
// from gen/ — single source of truth is proto/ (I1). Referenced here, never redefined.
package com.troubastack.shared.sync

/**
 * Sync client — SHARED Kotlin (commonMain). NOT a native seam (I15): a WebSocket and an outbox
 * are platform-agnostic. This is the in-app counterpart to the editor's optimistic state; it
 * exists for the app's own reconciliation needs (Studio in the webview has its own client too).
 *
 * Model (I6): the server (TroubaCore) is authoritative; the client is optimistic. Local changes
 * render immediately and live in an OUTBOX until the server accepts them. The server echoes
 * accepted mutations; the client applies echoes idempotently by uuid (I2) — the SAME apply path
 * handles own-echoes and peers' mutations. A rejection (deleted-remotely / stale-version, I5)
 * rolls back the optimistic change.
 *
 * No client is ever a source of truth. See docs/design/02-sync-protocol.md.
 */
interface SyncClient {

    /** Open the realtime room for a song (sends `subscribe_song_id`; expects a `Snapshot`). */
    suspend fun subscribe(songId: String) { TODO("scaffold: open WebSocket, send ClientMessage") }

    /**
     * Optimistically enqueue a mutation into the outbox and send it. Held until the server
     * echoes it (I6). `mutation` maps to gen proto `Mutation` (I1, client-generated uuid I2).
     */
    fun enqueue(mutation: /* Mutation */ Any) { TODO("scaffold: outbox + send") }

    /**
     * Sink for server messages (echo / rejection / snapshot). Apply echoes idempotently by
     * uuid (I2); roll back on rejection (I5).
     */
    fun onServerMessage(handler: (/* ServerMessage */ Any) -> Unit) { TODO("scaffold") }
}
