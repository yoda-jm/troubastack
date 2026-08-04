\version "2.24.0"
% Pachelbel — Canon in D (1680). PUBLIC DOMAIN. Violin I (the canon melody, first strain).
\header { title = "Canon in D" subtitle = "Pachelbel (1680) — public domain · Violin I" tagline = ##f }
\paper { #(set-paper-size "a4") ragged-bottom = ##t }
\score {
  \new Staff \with { instrumentName = "Violin I " } {
    \clef treble \key d \major \time 4/4 \tempo "Andante" 4 = 56
    fis''2 e''2  | d''2 cis''2 | b'2 a'2 | b'2 cis''2 |
    d''2 cis''2  | b'2 a'2 | g'2 fis'2 | g'2 e'2 \bar "|."
  }
  \layout {}
}
