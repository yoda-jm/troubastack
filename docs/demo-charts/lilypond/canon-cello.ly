\version "2.24.0"
% Pachelbel — Canon in D (1680). PUBLIC DOMAIN. Cello (the famous ground bass).
\header { title = "Canon in D" subtitle = "Pachelbel (1680) — public domain · Cello" tagline = ##f }
\paper { #(set-paper-size "a4") ragged-bottom = ##t }
\score {
  \new Staff \with { instrumentName = "Cello " } {
    \clef bass \key d \major \time 4/4 \tempo "Andante" 4 = 56
    d2 a,2 | b,2 fis,2 | g,2 d,2 | g,2 a,2 |
    d2 a,2 | b,2 fis,2 | g,2 d,2 | g,2 a,2 \bar "|."
  }
  \layout {}
}
