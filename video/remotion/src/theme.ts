// The brand palette (docs/brand): one arc colour per product layer.
export const C = {
  tile: "#202B36", tileHi: "#26333F", tileLo: "#1B242D",
  stage: "#FCCC55", studio: "#E13198", core: "#2A8FE9", hl: "#FEE963",
  paper: "#FBF9F5", paperAlt: "#F3EFE7", ink: "#141A1F", inkSoft: "#5C6670",
  onDark: "#F4F1EA", onDarkSoft: "#A7B0BA",
};
export const ARC: Record<string, string> = { stage: C.stage, studio: C.studio, core: C.core, stack: C.stage };
export const SERIF = "'Iowan Old Style','Palatino Linotype',Palatino,'Book Antiqua',Georgia,serif";
export const SANS = "system-ui,-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif";
export const MONO = "ui-monospace,SFMono-Regular,Menlo,Consolas,'Liberation Mono',monospace";
