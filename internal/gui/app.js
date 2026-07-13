const $ = (id) => document.getElementById(id);

document.querySelectorAll("nav button").forEach((b) => {
  b.onclick = () => {
    document.querySelectorAll("nav button").forEach((x) => x.classList.remove("active"));
    document.querySelectorAll("section").forEach((x) => x.classList.remove("active"));
    b.classList.add("active");
    $(b.dataset.tab).classList.add("active");
  };
});

async function api(path, method = "GET", body) {
  const opt = { method, headers: { "Content-Type": "application/json" } };
  if (body) opt.body = JSON.stringify(body);
  const r = await fetch("/api/" + path, opt);
  const j = await r.json();
  if (j.error) {
    alert(j.error);
    throw new Error(j.error);
  }
  return j;
}

async function refreshStatus() {
  const s = await api("status");
  $("status").textContent = JSON.stringify(s, null, 2);
  $("runstate").textContent =
    "mesh: " + (s.running ? "running" : "stopped") + " · neighbors " + (s.p2p_neighbors || 0);
}

async function loadIdentity() {
  try {
    $("ident").textContent = JSON.stringify(await api("identity"), null, 2);
  } catch (e) {
    $("ident").textContent = String(e);
  }
}

async function doInit() {
  const j = await api("init", "POST", { print_mnemonic: true });
  $("ident").textContent = JSON.stringify(j, null, 2);
  refreshStatus();
}

async function loadSeeds() {
  const j = await api("seeds");
  $("seedlist").textContent =
    (j.seeds || []).map((s, i) => i + 1 + ". " + s).join("\n") ||
    "(empty — built-in may apply)";
}

async function setSeed() {
  await api("seed/set", "POST", { url: $("seedurl").value });
  loadSeeds();
}
async function addSeed() {
  await api("seed/add", "POST", { url: $("seedurl").value });
  loadSeeds();
}
async function health() {
  $("healthout").textContent = JSON.stringify(await api("health"), null, 2);
}

async function loadPeers() {
  try {
    const j = await api("peers");
    let t = "P2P:\n";
    (j.punched || []).forEach((p) => {
      t += "  " + p.node_id + " @ " + p.addr + "\n";
    });
    t += "\nDiscovery:\n";
    (j.discovery || []).forEach((p) => {
      t += "  " + p.node_id + " " + JSON.stringify(p.candidates || []) + "\n";
    });
    $("peers").textContent = t;
  } catch (e) {
    $("peers").textContent = String(e);
  }
}

async function sendMsg() {
  const j = await api("send", "POST", {
    to: $("toid").value.trim(),
    text: $("msg").value,
  });
  alert("sent " + j.sha1);
  loadInbox();
}
async function broadcastMsg() {
  const j = await api("broadcast", "POST", { text: $("msg").value });
  alert("broadcast " + j.sha1);
  loadInbox();
}
async function loadInbox() {
  const rows = await api("inbox");
  $("inbox").innerHTML =
    (rows || [])
      .map(
        (m) =>
          '<div class="msg"><b>' +
          m.tag +
          '</b> from <span class="mono">' +
          m.from +
          "</span><br/>" +
          escapeHtml(m.text) +
          "</div>"
      )
      .join("") || '<div class="badge">empty</div>';
}
async function clearInbox() {
  await api("inbox/clear", "POST");
  loadInbox();
}
async function verifyFP() {
  await api("verify", "POST", {
    id: $("vid").value.trim(),
    fp: $("vfp").value.trim(),
  });
  loadTrust();
}
async function loadTrust() {
  $("trustlist").textContent = JSON.stringify(await api("trust"), null, 2);
}
function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

refreshStatus();
loadSeeds();
loadIdentity().catch(() => {});
setInterval(refreshStatus, 4000);
