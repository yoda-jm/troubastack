package com.troubastack.desktop.studio

import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * WIN08 S2.10 — the origin lock's off-origin veto is relaxed ONLY for a dispatch-only iframe probe: a subframe
 * navigation, with BOTH the iframe knob and the relax knob set. The decision lives in [s210RelaxSubframeVeto] so this
 * runs off a Windows host (the LocationListener that calls it is SWT, Windows-only). TEETH: the default path cannot
 * reach the relax even if the relax knob is set alone, and a top-frame navigation is never relaxed.
 */
class S210RelaxSubframeVetoTest {
    @Test
    fun bothKnobsAndSubframe_relaxes() {
        assertTrue(s210RelaxSubframeVeto(probeIframe = true, relaxIframe = true, isTop = false))
    }

    @Test
    fun relaxKnobAlone_doesNotRelax() {
        // The default path never sets the iframe probe knob; RELAX alone in the environment must change nothing.
        assertFalse(s210RelaxSubframeVeto(probeIframe = false, relaxIframe = true, isTop = false))
    }

    @Test
    fun defaultPath_noKnobs_doesNotRelax() {
        assertFalse(s210RelaxSubframeVeto(probeIframe = false, relaxIframe = false, isTop = false))
    }

    @Test
    fun iframeProbeWithoutRelax_doesNotRelax() {
        // Pass 1 (veto on): the iframe probe runs but must still veto the subframe.
        assertFalse(s210RelaxSubframeVeto(probeIframe = true, relaxIframe = false, isTop = false))
    }

    @Test
    fun topFrame_isNeverRelaxed_evenWithBothKnobs() {
        assertFalse(s210RelaxSubframeVeto(probeIframe = true, relaxIframe = true, isTop = true))
    }
}
