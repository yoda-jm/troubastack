\version "2.24.0"
% Eine kleine Nachtmusik (W. A. Mozart, K. 525, 1787). PUBLIC DOMAIN.
% Violin I — opening theme. BEST-EFFORT RECONSTRUCTION from memory: the rising
% G-major "rocket" figures + their descending answers, tonic then dominant.
% VERIFY the exact notes/rhythm against a score before recording.
\header {
  title = "Eine kleine Nachtmusik"
  subtitle = "W. A. Mozart (K. 525) — public domain · Violin I (opening)"
  tagline = ##f
}
\paper { #(set-paper-size "a4") ragged-bottom = ##t }
\score {
  \new Staff \with { instrumentName = "Violin I " } {
    \clef treble \key g \major \time 4/4 \tempo "Allegro" 4 = 140
    g'8\f d''8 g''4 g'8 d''8 g''4 |
    d''8 c''8 b'8 a'8 g'2 |
    a'8 d''8 a''4 a'8 d''8 a''4 |
    b''8 a''8 g''8 fis''8 g''2 \bar "|."
  }
  \layout {}
}
