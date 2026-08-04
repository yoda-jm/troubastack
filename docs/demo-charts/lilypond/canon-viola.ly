\version "2.24.0"
% Pachelbel — Canon in D (1680). PUBLIC DOMAIN. Viola (a simple inner harmony line).
\header { title = "Canon in D" subtitle = "Pachelbel (1680) — public domain · Viola" tagline = ##f }
\paper { #(set-paper-size "a4") ragged-bottom = ##t }
\score {
  \new Staff \with { instrumentName = "Viola " } {
    \clef alto \key d \major \time 4/4 \tempo "Andante" 4 = 56
    a'2 a'2 | fis'2 e'2 | d'2 d'2 | g'2 g'2 |
    fis'2 g'2 | d'2 fis'2 | e'2 d'2 | b2 cis'2 \bar "|."
  }
  \layout {}
}
