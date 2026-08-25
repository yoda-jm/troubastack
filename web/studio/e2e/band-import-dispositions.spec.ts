/**
 * T63: invite-on-import. Importing a .tband previews its members and lets the admin choose
 * per missing member — Create / Invite / Skip. This drives the Invite path end-to-end: the
 * member is NOT created but gets a pending invite they see on first sign-in.
 *
 * The e2e stack is a single server, so a normal export has no "missing" members (every
 * exported member exists here). We therefore hand-craft a minimal .tband whose one member
 * has no account on this server — exactly the case the dialog exists for.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register } from "./setup-helpers";

async function logout(page: Page) {
  await page.getByTestId("account-trigger").click();
  await page.getByTestId("logout").click();
  await expect(page).toHaveURL(/\/login$/);
}

// crc32 (IEEE, the zip polynomial) over a buffer.
function crc32(buf: Buffer): number {
  let c = ~0;
  for (let i = 0; i < buf.length; i++) {
    c ^= buf[i];
    for (let k = 0; k < 8; k++) c = (c >>> 1) ^ (0xedb88320 & -(c & 1));
  }
  return ~c >>> 0;
}

// tbandZip builds a real (stored, uncompressed) .tband zip with a single band.json entry —
// enough for the server to parse + validate + preview.
function tbandZip(manifest: object): Buffer {
  const name = Buffer.from("band.json");
  const data = Buffer.from(JSON.stringify(manifest));
  const crc = crc32(data);

  const lfh = Buffer.alloc(30 + name.length);
  lfh.writeUInt32LE(0x04034b50, 0);
  lfh.writeUInt16LE(20, 4); // version needed
  lfh.writeUInt16LE(0, 6); // flags
  lfh.writeUInt16LE(0, 8); // method: stored
  lfh.writeUInt32LE(crc, 14);
  lfh.writeUInt32LE(data.length, 18);
  lfh.writeUInt32LE(data.length, 22);
  lfh.writeUInt16LE(name.length, 26);
  name.copy(lfh, 30);

  const cdh = Buffer.alloc(46 + name.length);
  cdh.writeUInt32LE(0x02014b50, 0);
  cdh.writeUInt16LE(20, 4); // version made by
  cdh.writeUInt16LE(20, 6); // version needed
  cdh.writeUInt16LE(0, 10); // method: stored
  cdh.writeUInt32LE(crc, 16);
  cdh.writeUInt32LE(data.length, 20);
  cdh.writeUInt32LE(data.length, 24);
  cdh.writeUInt16LE(name.length, 28);
  name.copy(cdh, 46);

  const eocd = Buffer.alloc(22);
  eocd.writeUInt32LE(0x06054b50, 0);
  eocd.writeUInt16LE(1, 8); // entries this disk
  eocd.writeUInt16LE(1, 10); // entries total
  eocd.writeUInt32LE(cdh.length, 12); // cd size
  eocd.writeUInt32LE(lfh.length + data.length, 16); // cd offset (central dir starts after LFH+data)

  return Buffer.concat([lfh, data, cdh, eocd]);
}

test("import dialog invites a missing member; they see the invite on sign-in (T63)", async ({
  page,
}) => {
  await register(page, `adm_${stamp()}`);

  const bandName = `Ghost Import ${stamp()}`;
  const ghost = `ghost_${stamp()}`;
  const zip = tbandZip({
    formatVersion: 1,
    band: { name: bandName },
    members: [{ id: "m1", username: ghost, displayName: "Ghost Member", role: "member" }],
    songs: [],
    setlists: [],
  });

  await page
    .getByTestId("import-band-input")
    .setInputFiles({ name: "ghost.tband.zip", mimeType: "application/zip", buffer: zip });

  // The preview dialog lists the missing member; default disposition is Create.
  const dialog = page.getByTestId("import-dialog");
  await expect(dialog).toBeVisible();
  await expect(page.getByTestId("import-missing-list")).toContainText(ghost);
  await expect(page.getByTestId(`disposition-${ghost}`)).toHaveValue("create");

  // Choose Invite, then confirm.
  await page.getByTestId(`disposition-${ghost}`).selectOption("invite");
  await page.getByTestId("import-confirm").click();

  // Lands on the new band; the report shows the member was invited.
  await expect(page).toHaveURL(/\/bands\/[^/]+$/);
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  await expect(page.getByTestId("import-invited")).toContainText(ghost);

  // The invited member registers and sees the pending invite on their invites page.
  await logout(page);
  await register(page, ghost);
  await page.goto("/invites");
  await expect(page.getByTestId("invites-list")).toContainText(ghost);
});
