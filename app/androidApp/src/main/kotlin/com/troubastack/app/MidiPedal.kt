package com.troubastack.app

import android.Manifest
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothDevice
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanFilter
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.Context
import android.content.pm.PackageManager
import android.media.midi.MidiDevice
import android.media.midi.MidiManager
import android.media.midi.MidiReceiver
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelUuid
import androidx.core.content.ContextCompat
import com.troubastack.shared.stage.MidiPress
import com.troubastack.shared.stage.parseMidiPresses
import java.util.UUID

/**
 * A76 — the BLE-MIDI pedal client. Scans for the standard BLE-MIDI GATT service, opens it via the OS's
 * [MidiManager] (Fable A76 §5: let the OS own the link — the ~2 s disconnects under a generic central are the
 * reason, not the counter-argument), and forwards parsed PRESSES ([parseMidiPresses], the pure seam).
 *
 * Connection [State] drives the Learn panel's ⟨D1⟩ diagnostic: a disconnected pedal and a mute one must not
 * look alike. Everything Android is here; the matching/learning is pure + shared. Never throws (a value at
 * every edge); a missing permission / BT-off / no-MIDI-service just leaves it NOT_CONNECTED.
 */
class MidiPedal(private val context: Context) {
    enum class State { NOT_CONNECTED, SCANNING, CONNECTED }

    private val bleMidiService = UUID.fromString("03B80E5A-EDE8-4B33-A751-6CE34EC4C700")
    private val main = Handler(Looper.getMainLooper())
    private val midi = context.getSystemService(Context.MIDI_SERVICE) as? MidiManager
    private val btAdapter = (context.getSystemService(Context.BLUETOOTH_SERVICE) as? BluetoothManager)?.adapter

    @Volatile var state: State = State.NOT_CONNECTED
        private set
    private var onState: ((State) -> Unit)? = null
    private var onPress: ((MidiPress) -> Unit)? = null
    private var openDevice: MidiDevice? = null
    private var scanCb: ScanCallback? = null

    /** Wire the UI/Stage listeners; immediately replays the current state so the caller can render it. */
    fun setListeners(onStateChange: (State) -> Unit, onPressReceived: (MidiPress) -> Unit) {
        onState = onStateChange; onPress = onPressReceived; onStateChange(state)
    }

    /** True iff the runtime BLE permissions are held (Android 12+ needs SCAN + CONNECT; older needs none here). */
    fun hasPermissions(): Boolean =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S)
            ContextCompat.checkSelfPermission(context, Manifest.permission.BLUETOOTH_SCAN) == PackageManager.PERMISSION_GRANTED &&
                ContextCompat.checkSelfPermission(context, Manifest.permission.BLUETOOTH_CONNECT) == PackageManager.PERMISSION_GRANTED
        else true

    fun btReady(): Boolean = btAdapter?.isEnabled == true && midi != null

    private fun setState(s: State) { state = s; main.post { onState?.invoke(s) } }

    private var opening = false

    /**
     * Connect. Try the BONDED devices FIRST — VLL's pedal is paired, and `openBluetoothDevice` works on a
     * bonded device without it advertising the BLE-MIDI service (the service-filter scan never saw it: this
     * pedal doesn't put the MIDI UUID in its adv packet). The bonded device that exposes a MIDI output port is
     * the pedal. Only if none of the bonded devices are MIDI do we fall back to a scan (for an unpaired pedal).
     * Idempotent; no-op if busy or unready.
     */
    fun connect() {
        if (state == State.CONNECTED || scanCb != null || opening) return
        val adapter = btAdapter ?: return
        if (!adapter.isEnabled || midi == null || !hasPermissions()) return
        setState(State.SCANNING)
        val bonded = try {
            adapter.bondedDevices?.toList().orEmpty()
                .filter { runCatching { it.type != android.bluetooth.BluetoothDevice.DEVICE_TYPE_CLASSIC }.getOrDefault(true) }
        } catch (e: SecurityException) { emptyList() }
        tryBonded(bonded, 0)
    }

    /** Open each candidate bonded device via MidiManager; the first with a MIDI output port is the pedal. */
    private fun tryBonded(devices: List<BluetoothDevice>, idx: Int) {
        if (state == State.CONNECTED) return
        val m = midi ?: run { setState(State.NOT_CONNECTED); return }
        if (idx >= devices.size) { startScan(); return } // none were MIDI → scan as a fallback
        opening = true
        try {
            m.openBluetoothDevice(devices[idx], MidiManager.OnDeviceOpenedListener { device ->
                opening = false
                if (device != null && attach(device)) return@OnDeviceOpenedListener
                try { device?.close() } catch (_: Exception) {}
                tryBonded(devices, idx + 1)
            }, main)
        } catch (e: SecurityException) { opening = false; tryBonded(devices, idx + 1) }
    }

    /** Fallback: scan for anything advertising the BLE-MIDI service, open the first found. */
    private fun startScan() {
        val scanner = btAdapter?.bluetoothLeScanner ?: run { setState(State.NOT_CONNECTED); return }
        val filter = ScanFilter.Builder().setServiceUuid(ParcelUuid(bleMidiService)).build()
        val settings = ScanSettings.Builder().setScanMode(ScanSettings.SCAN_MODE_LOW_LATENCY).build()
        val cb = object : ScanCallback() {
            override fun onScanResult(callbackType: Int, result: ScanResult) { result.device?.let { stopScan(); openScanned(it) } }
            override fun onScanFailed(errorCode: Int) { stopScan(); setState(State.NOT_CONNECTED) }
        }
        scanCb = cb
        try {
            scanner.startScan(listOf(filter), settings, cb)
            // Fail visibly rather than hang on "searching…": no BLE-MIDI advertiser in 12 s → NOT_CONNECTED.
            main.postDelayed({ if (scanCb === cb) { stopScan(); setState(State.NOT_CONNECTED) } }, 12_000)
        } catch (e: SecurityException) { scanCb = null; setState(State.NOT_CONNECTED) }
    }

    private fun stopScan() {
        val cb = scanCb ?: return
        scanCb = null
        try { btAdapter?.bluetoothLeScanner?.stopScan(cb) } catch (_: SecurityException) {}
    }

    private fun openScanned(dev: BluetoothDevice) {
        val m = midi ?: run { setState(State.NOT_CONNECTED); return }
        try {
            m.openBluetoothDevice(dev, MidiManager.OnDeviceOpenedListener { device ->
                if (device == null || !attach(device)) { try { device?.close() } catch (_: Exception) {}; setState(State.NOT_CONNECTED) }
            }, main)
        } catch (e: SecurityException) { setState(State.NOT_CONNECTED) }
    }

    /** Open the device's MIDI output port + attach the receiver. Returns true iff it exposed a MIDI output. */
    private fun attach(device: MidiDevice): Boolean {
        if (device.info.outputPortCount <= 0) return false
        val port = try { device.openOutputPort(0) } catch (e: SecurityException) { null } ?: return false
        openDevice = device
        port.connect(object : MidiReceiver() {
            override fun onSend(msg: ByteArray, offset: Int, count: Int, timestamp: Long) {
                val presses = parseMidiPresses(msg, offset, count)
                if (presses.isNotEmpty()) main.post { presses.forEach { p -> onPress?.invoke(p) } }
            }
        })
        setState(State.CONNECTED)
        return true
    }

    /** Tear down (leaving Stage / forgetting the pedal). */
    fun close() {
        stopScan()
        try { openDevice?.close() } catch (_: Exception) {}
        openDevice = null
        setState(State.NOT_CONNECTED)
    }
}
