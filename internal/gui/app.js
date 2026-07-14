const $ = (id) => document.getElementById(id);

document.querySelectorAll("nav button").forEach((b) => {
  b.onclick = () => {
    document.querySelectorAll("nav button").forEach((x) => x.classList.remove("active"));
    document.querySelectorAll("section").forEach((x) => x.classList.remove("active"));
    b.classList.add("active");
    $(b.dataset.tab).classList.add("active");
    if (b.dataset.tab === "wallet") loadChain();
  };
});

async function api(path, method = "GET", body) {
  const opt = { method, headers: { "Content-Type": "application/json" } };
  if (body !== undefined && body !== null) {
    opt.body = JSON.stringify(body);
  }
  let r;
  try {
    r = await fetch("/api/" + path, opt);
  } catch (e) {
    const msg = "network error: " + e;
    alert(msg);
    throw new Error(msg);
  }
  const text = await r.text();
  let j = null;
  if (text && text.trim() !== "") {
    try {
      j = JSON.parse(text);
    } catch (e) {
      const msg = "invalid JSON (HTTP " + r.status + "): " + text.slice(0, 180);
      alert(msg);
      throw new Error(msg);
    }
  }
  // empty body → treat as {}
  if (j === null || j === undefined) {
    j = {};
  }
  // inbox returns a JSON array
  if (Array.isArray(j)) {
    if (!r.ok) {
      const msg = "HTTP " + r.status;
      alert(msg);
      throw new Error(msg);
    }
    return j;
  }
  if (typeof j !== "object") {
    const msg = "unexpected response type";
    alert(msg);
    throw new Error(msg);
  }
  if (j.error || !r.ok) {
    const msg = j.error || "HTTP " + r.status;
    alert(msg);
    throw new Error(msg);
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
  try {
    const to = $("toid").value.trim();
    const text = $("msg").value;
    if (!to || !text) {
      alert("请填写对方 Node ID 和消息内容");
      return;
    }
    const j = await api("send", "POST", { to, text });
    alert("sent " + (j.sha1 || "ok"));
    await loadInbox();
  } catch (e) {
    // api() already alerted; avoid uncaught rejection
  }
}
async function broadcastMsg() {
  try {
    const j = await api("broadcast", "POST", { text: $("msg").value });
    alert("broadcast " + (j.sha1 || "ok"));
    await loadInbox();
  } catch (e) {}
}
async function loadInbox() {
  try {
    const rows = await api("inbox");
    const list = Array.isArray(rows) ? rows : [];
    $("inbox").innerHTML =
      list
        .map(
          (m) =>
            '<div class="msg"><b>' +
            (m.tag || "dm") +
            '</b> from <span class="mono">' +
            (m.from || "") +
            "</span><br/>" +
            escapeHtml(m.text || "") +
            "</div>"
        )
        .join("") || '<div class="badge">empty</div>';
  } catch (e) {
    $("inbox").innerHTML =
      '<div class="badge">inbox unavailable: ' + escapeHtml(String(e)) + "</div>";
  }
}
async function clearInbox() {
  try {
    await api("inbox/clear", "POST");
    await loadInbox();
  } catch (e) {}
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

async function loadChain() {
  try {
    const j = await api("chain");
    const lines = [
      "chain_id:     " + (j.chain_id || ""),
      "network:      " + (j.network || ""),
      "consensus:    " + (j.consensus || ""),
      "height:       " + j.height,
      "tip_slot:     " + j.tip_slot,
      "my_balance:   " + j.my_balance + " MST  (liquid)",
      "my_stake:     " + j.my_stake + " MST",
      "my_active:    " + j.my_active,
      "pool_left:    " + j.reward_pool_remaining,
      "validators:   " + j.validators,
      "---",
      JSON.stringify(j, null, 2),
    ];
    $("chainbal").textContent = lines.join("\n");
  } catch (e) {
    $("chainbal").textContent = String(e);
  }
}

async function doTransfer() {
  const to = $("xferto").value.trim();
  const amount = parseInt($("xferamt").value, 10);
  if (!to || !amount || amount < 1) {
    alert("请填写收款 Node ID 和正整数金额");
    return;
  }
  if (
    !confirm(
      "确认转账？\n收款: " +
        to +
        "\n金额: " +
        amount +
        " MST\n手续费: 1 MST\n合计扣除: " +
        (amount + 1) +
        " MST"
    )
  ) {
    return;
  }
  try {
    const j = await api("chain/transfer", "POST", { to, amount });
    $("xferout").textContent =
      "已提交转账: " + amount + " MST → " + to + "\n" + JSON.stringify(j, null, 2);
    // try pack into a block so it confirms on solo node
    try {
      const m = await api("chain/mine", "POST", { force: true });
      $("xferout").textContent +=
        "\n已提议出块 h=" + m.height + " slot=" + m.slot + " hash=" + (m.hash || "").slice(0, 16);
    } catch (mineErr) {
      $("xferout").textContent +=
        "\n出块提示: " + mineErr + "（交易可能在 mempool，稍后 chain-mine）";
    }
    await loadChain();
  } catch (e) {
    $("xferout").textContent = "失败: " + e;
  }
}

async function chainMine() {
  try {
    const j = await api("chain/mine", "POST", { force: true });
    alert("proposed h=" + j.height + " slot=" + j.slot);
    loadChain();
  } catch (e) {
    /* alert already from api */
  }
}

async function chainActivate() {
  try {
    await api("chain/activate", "POST", {});
    alert("validator activated");
    loadChain();
  } catch (e) {}
}

async function doStake() {
  const amount = parseInt($("stakeamt").value, 10);
  if (!amount || amount < 1) {
    alert("请输入质押数量");
    return;
  }
  try {
    await api("chain/stake", "POST", { amount });
    alert("staked " + amount);
    loadChain();
  } catch (e) {}
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

refreshStatus().catch(() => {});
loadSeeds().catch(() => {});
loadIdentity().catch(() => {});
setInterval(() => {
  refreshStatus().catch(() => {});
}, 4000);
