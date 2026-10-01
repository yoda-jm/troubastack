// T180 — the song tag field: a chip input whose commit gesture is the delimiter (⟨D1⟩), a suggestion list
// carrying each band tag's usage count (⟨D2⟩), and the band vocabulary as a bounded clickable cloud (⟨D3⟩).
// All the rules live in ./tagInput (pure, unit-tested); this file is the DOM around them.
import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "../../api";
import {
  backspaceLast,
  cloud,
  CLOUD_DEFAULT_N,
  commitAll,
  commitBuffer,
  suggest,
  type Vocab,
} from "./tagInput";

export function TagInput({
  bandId,
  value,
  onChange,
}: {
  bandId: string;
  value: string[];
  onChange: (tags: string[]) => void;
}) {
  const [buffer, setBuffer] = useState("");
  const [vocab, setVocab] = useState<Vocab>([]);
  const [showAll, setShowAll] = useState(false);
  const [open, setOpen] = useState(false); // suggestion list visibility
  const inputRef = useRef<HTMLInputElement | null>(null);

  // ⟨D4⟩ one call per editor; a failure leaves the cloud empty but the field fully usable.
  useEffect(() => {
    let live = true;
    api
      .bandTags(bandId)
      .then((t) => live && setVocab(t))
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [bandId]);

  const { matches, createLabel } = useMemo(
    () => suggest(buffer, vocab, value),
    [buffer, vocab, value],
  );
  const { shown, hidden } = useMemo(
    () => cloud(vocab, value, CLOUD_DEFAULT_N, showAll),
    [vocab, value, showAll],
  );

  // Fable T180 ⟨1⟩ — a mousedown on ANY control inside the field must not let the input blur, or onBlur
  // commits the half-typed buffer as a side effect and a click on a cloud tag saves the fragment beside it.
  // preventDefault keeps focus on the input; the control's own onClick still fires. Applied to every
  // interactive control in the field (chips' ×, suggestions, create, cloud items, "+N more"), so the
  // property holds on all of them, not only the path that was measured.
  const keepFocus = (e: React.MouseEvent) => e.preventDefault();

  function add(raw: string) {
    onChange(commitAll(raw, value, vocab));
    setBuffer("");
    setOpen(false);
    inputRef.current?.focus();
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter") {
      e.preventDefault(); // never submit the surrounding form on a tag commit
      if (buffer.trim() !== "") add(buffer);
      return;
    }
    if (e.key === "," ) {
      // The comma itself is the delimiter, not content: commit the word(s) before it and keep the tail.
      e.preventDefault();
      const step = commitBuffer(buffer + ",", value, vocab);
      onChange(step.tags);
      setBuffer(step.buffer);
      return;
    }
    if (e.key === "Backspace" && buffer === "" && value.length > 0) {
      // Remove exactly the last chip — never the one before it.
      onChange(backspaceLast(value));
    }
  }

  function onPaste(e: React.ClipboardEvent<HTMLInputElement>) {
    const text = e.clipboardData.getData("text");
    if (!text.includes(",")) return; // a plain word: let it land in the buffer normally
    e.preventDefault();
    const step = commitBuffer(buffer + text, value, vocab);
    const tags = step.buffer.trim() === "" ? step.tags : commitAll(step.buffer, step.tags, vocab);
    onChange(tags);
    setBuffer("");
  }

  return (
    <div className="tag-input" data-testid="tag-input">
      <div className="tag-chips" data-testid="tag-chips">
        {value.map((t) => (
          <span className="tag-chip" data-testid="tag-chip" key={t}>
            {t}
            <button
              type="button"
              className="tag-chip-x"
              aria-label={`Remove ${t}`}
              data-testid="tag-chip-remove"
              onMouseDown={keepFocus}
              onClick={() => onChange(value.filter((x) => x !== t))}
            >
              ×
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          className="tag-buffer"
          data-testid="tag-buffer"
          aria-label="Tags"
          value={buffer}
          placeholder={value.length === 0 ? "Add a tag…" : ""}
          onChange={(e) => {
            setBuffer(e.target.value);
            setOpen(true);
          }}
          onKeyDown={onKeyDown}
          onPaste={onPaste}
          onFocus={() => setOpen(true)}
          // A blur commits an in-progress word so it is not silently lost, then closes the list.
          onBlur={() => {
            if (buffer.trim() !== "") add(buffer);
            else setOpen(false);
          }}
        />
      </div>

      {open && (matches.length > 0 || createLabel) && (
        <ul className="tag-suggestions" data-testid="tag-suggestions" role="listbox">
          {matches.map((m) => (
            <li key={m.tag}>
              <button type="button" data-testid="tag-suggestion" onMouseDown={keepFocus} onClick={() => add(m.tag)}>
                <span className="tag-suggestion-name">{m.tag}</span>
                {/* ⟨D2⟩ the count is the feature: it tells a convention from a typo at the moment of typing. */}
                <span className="tag-suggestion-count" data-testid="tag-suggestion-count">
                  {m.count}
                </span>
              </button>
            </li>
          ))}
          {createLabel && (
            <li>
              <button
                type="button"
                className="tag-suggestion-create"
                data-testid="tag-create"
                onMouseDown={keepFocus}
                onClick={() => add(createLabel)}
              >
                Create “{createLabel}”
              </button>
            </li>
          )}
        </ul>
      )}

      {shown.length > 0 && (
        <div className="tag-cloud" data-testid="tag-cloud">
          {shown.map((v) => (
            <button
              type="button"
              className="tag-cloud-item"
              data-testid="tag-cloud-item"
              key={v.tag}
              onMouseDown={keepFocus}
              onClick={() => add(v.tag)}
            >
              {v.tag} <span className="tag-cloud-count">{v.count}</span>
            </button>
          ))}
          {hidden > 0 && (
            <button
              type="button"
              className="tag-cloud-more"
              data-testid="tag-cloud-more"
              onMouseDown={keepFocus}
              onClick={() => setShowAll(true)}
            >
              +{hidden} more
            </button>
          )}
        </div>
      )}
    </div>
  );
}
