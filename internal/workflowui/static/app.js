(() => {
  const state = {
    meta: null,
    tools: [],
    workflows: [],
    currentName: null,
    dirty: false,
    selectedId: null,
    selectedEdge: null,
    view: { x: 0, y: 0, scale: 1 },
    draft: emptyWorkflow(),
    layout: { nodes: {} },
    drag: null,
    link: null,
    pan: null,
  };

  const el = {
    list: document.getElementById("workflow-list"),
    metaFoot: document.getElementById("meta-foot"),
    canvas: document.getElementById("canvas"),
    edges: document.getElementById("edges"),
    wrap: document.getElementById("canvas-wrap"),
    status: document.getElementById("status"),
    name: document.getElementById("wf-name"),
    desc: document.getElementById("wf-desc"),
    mode: document.getElementById("wf-mode"),
    scope: document.getElementById("wf-scope"),
    form: document.getElementById("inspector-form"),
    empty: document.getElementById("inspector-empty"),
    nodeId: document.getElementById("node-id"),
    nodeDesc: document.getElementById("node-desc"),
    nodeDiff: document.getElementById("node-diff"),
    toolsToggle: document.getElementById("node-tools-toggle"),
    toolsPanel: document.getElementById("node-tools-panel"),
    depsToggle: document.getElementById("node-deps-toggle"),
    depsPanel: document.getElementById("node-deps-panel"),
    nodePrompt: document.getElementById("node-prompt"),
  };

  function emptyWorkflow() {
    return {
      name: "new-workflow",
      description: "Describe this workflow",
      execution_mode: "auto",
      tasks: [
        {
          id: "step-1",
          description: "First step",
          prompt: "Do the first step. Args: {{args}}",
          difficulty: "easy",
          allowed_tools: [],
          depends_on: [],
        },
      ],
    };
  }

  function setStatus(msg, kind = "") {
    el.status.textContent = msg || "";
    el.status.className = "status" + (kind ? " " + kind : "");
  }

  async function api(path, opts) {
    const res = await fetch(path, opts);
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || res.statusText);
    }
    if (res.status === 204) return null;
    return res.json();
  }

  function markDirty() {
    state.dirty = true;
  }

  function taskById(id) {
    return state.draft.tasks.find((t) => t.id === id);
  }

  function ensureLayout() {
    if (!state.layout.nodes) state.layout.nodes = {};
    state.draft.tasks.forEach((t, i) => {
      if (!state.layout.nodes[t.id]) {
        state.layout.nodes[t.id] = { x: 80 + (i % 3) * 280, y: 80 + Math.floor(i / 3) * 140 };
      }
    });
  }

  function applyView() {
    const t = `translate(${state.view.x}px, ${state.view.y}px) scale(${state.view.scale})`;
    el.canvas.style.transform = t;
    el.edges.style.transform = t;
    el.edges.style.transformOrigin = "0 0";
  }

  function renderList() {
    el.list.innerHTML = "";
    if (!state.workflows.length) {
      el.list.innerHTML = `<div class="muted" style="padding:8px 12px">No workflows loaded.</div>`;
      return;
    }
    state.workflows.forEach((wf) => {
      const item = document.createElement("div");
      item.className = "list-item" + (wf.name === state.currentName ? " active" : "");
      item.innerHTML = `<div class="name">${escapeHtml(wf.name)}</div><div class="desc">${escapeHtml(wf.description || `${(wf.tasks || []).length} tasks`)}</div>`;
      item.onclick = () => loadWorkflow(wf.name);
      el.list.appendChild(item);
    });
  }

  function renderNodes() {
    ensureLayout();
    el.canvas.innerHTML = "";
    state.draft.tasks.forEach((task) => {
      const pos = state.layout.nodes[task.id] || { x: 80, y: 80 };
      const node = document.createElement("div");
      node.className = "node" + (state.selectedId === task.id ? " selected" : "");
      node.dataset.id = task.id;
      node.style.left = pos.x + "px";
      node.style.top = pos.y + "px";
      const diff = task.difficulty || "default";
      node.innerHTML = `
        <div class="node-header"><span>${escapeHtml(task.id)}</span><span class="node-badge">${escapeHtml(diff)}</span></div>
        <div class="node-body">${escapeHtml(task.description || "")}</div>
        <div class="node-ports">
          <div class="port in" data-port="in" data-id="${escapeAttr(task.id)}" title="input"></div>
          <div class="port out" data-port="out" data-id="${escapeAttr(task.id)}" title="output"></div>
        </div>`;
      node.addEventListener("mousedown", onNodeMouseDown);
      node.querySelectorAll(".port").forEach((p) => p.addEventListener("mousedown", onPortMouseDown));
      el.canvas.appendChild(node);
    });
    renderEdges();
    renderInspector();
  }

  function nodeCenter(id, side) {
    const pos = state.layout.nodes[id] || { x: 0, y: 0 };
    const w = 220, h = 88;
    return {
      x: side === "out" ? pos.x + w : pos.x,
      y: pos.y + h / 2,
    };
  }

  function bezier(a, b) {
    const dx = Math.max(40, Math.abs(b.x - a.x) * 0.5);
    return `M ${a.x} ${a.y} C ${a.x + dx} ${a.y}, ${b.x - dx} ${b.y}, ${b.x} ${b.y}`;
  }

  function renderEdges() {
    const parts = [];
    state.draft.tasks.forEach((task) => {
      (task.depends_on || []).forEach((dep) => {
        if (!taskById(dep)) return;
        const a = nodeCenter(dep, "out");
        const b = nodeCenter(task.id, "in");
        const d = bezier(a, b);
        const key = dep + "->" + task.id;
        const active = state.selectedEdge === key ? " active" : "";
        parts.push(`<path class="edge-hit" data-edge="${escapeAttr(key)}" d="${d}"></path>`);
        parts.push(`<path class="edge-path${active}" data-edge="${escapeAttr(key)}" d="${d}"></path>`);
      });
    });
    if (state.link) {
      const a = nodeCenter(state.link.from, "out");
      const b = state.link.cursor;
      parts.push(`<path class="edge-path active" d="${bezier(a, b)}"></path>`);
    }
    el.edges.innerHTML = parts.join("");
    el.edges.querySelectorAll(".edge-hit").forEach((p) => {
      p.addEventListener("mousedown", (e) => {
        e.stopPropagation();
        state.selectedEdge = p.dataset.edge;
        state.selectedId = null;
        renderNodes();
      });
    });
  }

  function renderInspector() {
    const task = state.selectedId ? taskById(state.selectedId) : null;
    if (!task) {
      el.form.classList.add("hidden");
      el.empty.classList.remove("hidden");
      closeMultiSelects();
      return;
    }
    el.empty.classList.add("hidden");
    el.form.classList.remove("hidden");
    el.nodeId.value = task.id || "";
    el.nodeDesc.value = task.description || "";
    el.nodeDiff.value = task.difficulty || "";
    el.nodePrompt.value = task.prompt || "";
    renderToolOptions(task.allowed_tools || []);
    renderDepOptions(task.depends_on || []);
  }

  // Built-in tools that only inspect state / files (IsReadOnly=true).
  // Skill is intentionally excluded from presets (meta tool).
  const READ_ONLY_TOOLS = new Set([
    "Diff",
    "Fetch",
    "Glob",
    "Grep",
    "LS",
    "LSP",
    "ReadMemory",
    "View",
    "ViewImage",
    "WebSearch",
  ]);

  // Meta / orchestration tools excluded from 读写 preset.
  const PRESET_EXCLUDE_TOOLS = new Set(["AskUser", "Skill", "Task"]);

  function isMCPTool(name) {
    return String(name || "").startsWith("mcp__");
  }

  function isPresetExcluded(name) {
    return PRESET_EXCLUDE_TOOLS.has(name) || isMCPTool(name);
  }

  function toolOptionsList(selected) {
    return uniqueList([...(state.tools || []), ...(selected || [])]);
  }

  function presetReadOnly(options) {
    return options.filter((name) => READ_ONLY_TOOLS.has(name) && !isPresetExcluded(name));
  }

  // 读写 = all available coding tools, minus MCP / Skill / AskUser / Task.
  function presetReadWrite(options) {
    return options.filter((name) => !isPresetExcluded(name));
  }

  function applyToolSelection(names) {
    const task = taskById(state.selectedId);
    if (!task) return;
    const selected = uniqueList(names || []);
    task.allowed_tools = selected;
    renderToolOptions(selected);
    markDirty();
    renderNodesKeepInspector();
  }

  function renderToolOptions(selected) {
    const selectedSet = new Set(selected || []);
    // Keep unknown tools that are already on the task.
    const options = toolOptionsList(selected);
    const listHTML = options.length
      ? options.map((name) => multiOptionHTML("tool", name, selectedSet.has(name))).join("")
      : `<div class="multi-select-empty">No tools available</div>`;
    el.toolsPanel.innerHTML = `
      <div class="multi-select-actions">
        <button type="button" class="chip" data-preset="readonly" title="Select read-only tools">只读</button>
        <button type="button" class="chip" data-preset="readwrite" title="Select read/write tools (exclude MCP, Skill, AskUser, Task)">读写</button>
        <button type="button" class="chip" data-preset="clear" title="Clear selection">清空</button>
      </div>
      <div class="multi-select-list">${listHTML}</div>`;
    el.toolsToggle.textContent = summarizeSelection(selected, "Select tools…", "tools");

    el.toolsPanel.querySelectorAll("[data-preset]").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        const kind = btn.getAttribute("data-preset");
        if (kind === "readonly") {
          applyToolSelection(presetReadOnly(options));
          return;
        }
        if (kind === "readwrite") {
          applyToolSelection(presetReadWrite(options));
          return;
        }
        if (kind === "clear") {
          applyToolSelection([]);
        }
      });
    });

    el.toolsPanel.querySelectorAll("input[type=checkbox]").forEach((input) => {
      input.addEventListener("change", () => {
        const task = taskById(state.selectedId);
        if (!task) return;
        task.allowed_tools = readChecked(el.toolsPanel);
        el.toolsToggle.textContent = summarizeSelection(task.allowed_tools, "Select tools…", "tools");
        markDirty();
        renderNodesKeepInspector();
      });
    });
  }

  function renderDepOptions(selected) {
    const selectedSet = new Set(selected || []);
    const options = state.draft.tasks
      .map((t) => t.id)
      .filter((id) => id && id !== state.selectedId);
    el.depsPanel.innerHTML = options.length
      ? options.map((name) => multiOptionHTML("dep", name, selectedSet.has(name))).join("")
      : `<div class="multi-select-empty">No other nodes</div>`;
    el.depsToggle.textContent = summarizeSelection(selected, "Select dependencies…", "deps");
    el.depsPanel.querySelectorAll("input[type=checkbox]").forEach((input) => {
      input.addEventListener("change", () => {
        const task = taskById(state.selectedId);
        if (!task) return;
        task.depends_on = readChecked(el.depsPanel);
        el.depsToggle.textContent = summarizeSelection(task.depends_on, "Select dependencies…", "deps");
        markDirty();
        renderNodesKeepInspector();
      });
    });
  }

  function multiOptionHTML(kind, name, checked) {
    return `<label class="multi-select-item">
      <input type="checkbox" data-kind="${kind}" value="${escapeAttr(name)}" ${checked ? "checked" : ""} />
      <span>${escapeHtml(name)}</span>
    </label>`;
  }

  function readChecked(panel) {
    return Array.from(panel.querySelectorAll("input[type=checkbox]:checked")).map((el) => el.value);
  }

  function summarizeSelection(values, emptyLabel, unit) {
    const list = values || [];
    if (!list.length) return emptyLabel;
    if (list.length <= 2) return list.join(", ");
    return `${list.length} ${unit} selected`;
  }

  function uniqueList(values) {
    const out = [];
    const seen = new Set();
    values.forEach((v) => {
      const s = String(v || "").trim();
      if (!s || seen.has(s)) return;
      seen.add(s);
      out.push(s);
    });
    out.sort((a, b) => a.localeCompare(b));
    return out;
  }

  function closeMultiSelects() {
    el.toolsPanel.classList.add("hidden");
    el.depsPanel.classList.add("hidden");
  }

  function togglePanel(panel) {
    const opening = panel.classList.contains("hidden");
    closeMultiSelects();
    if (opening) panel.classList.remove("hidden");
  }

  function renderNodesKeepInspector() {
    // lightweight node redraw used when only tools/deps change
    ensureLayout();
    const selected = state.selectedId;
    const edge = state.selectedEdge;
    // preserve open dropdown state
    const toolsOpen = !el.toolsPanel.classList.contains("hidden");
    const depsOpen = !el.depsPanel.classList.contains("hidden");
    el.canvas.innerHTML = "";
    state.draft.tasks.forEach((task) => {
      const pos = state.layout.nodes[task.id] || { x: 80, y: 80 };
      const node = document.createElement("div");
      node.className = "node" + (selected === task.id ? " selected" : "");
      node.dataset.id = task.id;
      node.style.left = pos.x + "px";
      node.style.top = pos.y + "px";
      const diff = task.difficulty || "default";
      node.innerHTML = `
        <div class="node-header"><span>${escapeHtml(task.id)}</span><span class="node-badge">${escapeHtml(diff)}</span></div>
        <div class="node-body">${escapeHtml(task.description || "")}</div>
        <div class="node-ports">
          <div class="port in" data-port="in" data-id="${escapeAttr(task.id)}" title="input"></div>
          <div class="port out" data-port="out" data-id="${escapeAttr(task.id)}" title="output"></div>
        </div>`;
      node.addEventListener("mousedown", onNodeMouseDown);
      node.querySelectorAll(".port").forEach((p) => p.addEventListener("mousedown", onPortMouseDown));
      el.canvas.appendChild(node);
    });
    state.selectedId = selected;
    state.selectedEdge = edge;
    renderEdges();
    if (toolsOpen) el.toolsPanel.classList.remove("hidden");
    if (depsOpen) el.depsPanel.classList.remove("hidden");
  }

  function syncTopbarFromDraft() {
    el.name.value = state.draft.name || "";
    el.desc.value = state.draft.description || "";
    el.mode.value = state.draft.execution_mode || "auto";
  }

  function syncDraftFromTopbar() {
    state.draft.name = (el.name.value || "").trim().toLowerCase().replace(/\s+/g, "-");
    state.draft.description = el.desc.value.trim();
    state.draft.execution_mode = el.mode.value;
  }

  function selectNode(id) {
    state.selectedId = id;
    state.selectedEdge = null;
    renderNodes();
  }

  function onNodeMouseDown(e) {
    if (e.button !== 0) return;
    if (e.target.classList.contains("port")) return;
    const id = e.currentTarget.dataset.id;
    selectNode(id);
    const pos = state.layout.nodes[id];
    state.drag = {
      id,
      startX: e.clientX,
      startY: e.clientY,
      origX: pos.x,
      origY: pos.y,
    };
    e.preventDefault();
  }

  function onPortMouseDown(e) {
    e.stopPropagation();
    e.preventDefault();
    const port = e.currentTarget.dataset.port;
    const id = e.currentTarget.dataset.id;
    if (port !== "out") return;
    const cursor = clientToWorld(e.clientX, e.clientY);
    state.link = { from: id, cursor };
    state.selectedEdge = null;
    renderEdges();
  }

  function clientToWorld(clientX, clientY) {
    const rect = el.wrap.getBoundingClientRect();
    return {
      x: (clientX - rect.left - state.view.x) / state.view.scale,
      y: (clientY - rect.top - state.view.y) / state.view.scale,
    };
  }

  function onMouseMove(e) {
    if (state.drag) {
      const dx = (e.clientX - state.drag.startX) / state.view.scale;
      const dy = (e.clientY - state.drag.startY) / state.view.scale;
      state.layout.nodes[state.drag.id] = {
        x: state.drag.origX + dx,
        y: state.drag.origY + dy,
      };
      markDirty();
      const node = el.canvas.querySelector(`.node[data-id="${cssEscape(state.drag.id)}"]`);
      if (node) {
        node.style.left = state.layout.nodes[state.drag.id].x + "px";
        node.style.top = state.layout.nodes[state.drag.id].y + "px";
      }
      renderEdges();
      return;
    }
    if (state.link) {
      state.link.cursor = clientToWorld(e.clientX, e.clientY);
      renderEdges();
      return;
    }
    if (state.pan) {
      state.view.x = state.pan.origX + (e.clientX - state.pan.startX);
      state.view.y = state.pan.origY + (e.clientY - state.pan.startY);
      applyView();
    }
  }

  function onMouseUp(e) {
    if (state.link) {
      const target = document.elementFromPoint(e.clientX, e.clientY);
      const port = target && target.classList && target.classList.contains("port") ? target : null;
      if (port && port.dataset.port === "in") {
        const to = port.dataset.id;
        const from = state.link.from;
        if (to && from && to !== from) {
          const task = taskById(to);
          if (task) {
            task.depends_on = task.depends_on || [];
            if (!task.depends_on.includes(from)) {
              task.depends_on.push(from);
              markDirty();
              setStatus(`Linked ${from} → ${to}`, "ok");
            }
          }
        }
      }
      state.link = null;
      renderNodes();
    }
    state.drag = null;
    state.pan = null;
  }

  function addNode() {
    let n = 1;
    const used = new Set(state.draft.tasks.map((t) => t.id));
    while (used.has("step-" + n)) n++;
    const id = "step-" + n;
    state.draft.tasks.push({
      id,
      description: "New step",
      prompt: "Describe what this step should do. Args: {{args}}",
      difficulty: "easy",
      allowed_tools: [],
      depends_on: [],
    });
    state.layout.nodes[id] = {
      x: 120 + (state.draft.tasks.length % 4) * 40,
      y: 120 + (state.draft.tasks.length % 4) * 40,
    };
    markDirty();
    selectNode(id);
    setStatus("Node added", "ok");
  }

  function deleteSelected() {
    if (state.selectedEdge) {
      const [from, to] = state.selectedEdge.split("->");
      const task = taskById(to);
      if (task) {
        task.depends_on = (task.depends_on || []).filter((d) => d !== from);
        markDirty();
      }
      state.selectedEdge = null;
      renderNodes();
      setStatus("Edge removed", "ok");
      return;
    }
    if (!state.selectedId) return;
    if (state.draft.tasks.length <= 1) {
      setStatus("Keep at least one node", "err");
      return;
    }
    const removed = state.selectedId;
    state.draft.tasks = state.draft.tasks.filter((t) => t.id !== removed);
    state.draft.tasks.forEach((t) => {
      t.depends_on = (t.depends_on || []).filter((d) => d !== removed);
    });
    delete state.layout.nodes[removed];
    state.selectedId = state.draft.tasks[0]?.id || null;
    markDirty();
    renderNodes();
    setStatus("Node deleted", "ok");
  }

  function autoLayout() {
    // simple level layout based on depends_on
    const ids = state.draft.tasks.map((t) => t.id);
    const levelOf = {};
    const byId = Object.fromEntries(state.draft.tasks.map((t) => [t.id, t]));
    let guard = 0;
    while (Object.keys(levelOf).length < ids.length && guard++ < 100) {
      ids.forEach((id) => {
        if (levelOf[id] != null) return;
        const deps = (byId[id].depends_on || []).filter((d) => byId[d]);
        if (deps.every((d) => levelOf[d] != null)) {
          levelOf[id] = deps.length ? Math.max(...deps.map((d) => levelOf[d])) + 1 : 0;
        }
      });
    }
    const buckets = {};
    ids.forEach((id) => {
      const lvl = levelOf[id] ?? 0;
      buckets[lvl] = buckets[lvl] || [];
      buckets[lvl].push(id);
    });
    Object.keys(buckets).forEach((lvl) => {
      buckets[lvl].forEach((id, i) => {
        state.layout.nodes[id] = { x: 80 + Number(lvl) * 280, y: 80 + i * 140 };
      });
    });
    markDirty();
    renderNodes();
    setStatus("Auto layout applied", "ok");
  }

  async function refreshList() {
    state.workflows = await api("/api/workflows");
    renderList();
  }

  async function loadWorkflow(name) {
    const wf = await api("/api/workflows/" + encodeURIComponent(name));
    state.currentName = wf.name;
    state.draft = {
      name: wf.name,
      description: wf.description || "",
      execution_mode: wf.execution_mode || "auto",
      tasks: (wf.tasks || []).map(normalizeTask),
    };
    state.layout = wf.layout || { nodes: {} };
    state.selectedId = state.draft.tasks[0]?.id || null;
    state.selectedEdge = null;
    state.dirty = false;
    syncTopbarFromDraft();
    renderList();
    renderNodes();
    setStatus("Loaded " + wf.name, "ok");
  }

  function normalizeTask(t, i) {
    return {
      id: t.id || "task-" + (i + 1),
      description: t.description || "",
      prompt: t.prompt || "",
      difficulty: t.difficulty || "",
      model: t.model || "",
      allowed_tools: t.allowed_tools || [],
      depends_on: t.depends_on || [],
    };
  }

  function newWorkflow() {
    state.currentName = null;
    state.draft = emptyWorkflow();
    state.layout = { nodes: { "step-1": { x: 120, y: 120 } } };
    state.selectedId = "step-1";
    state.selectedEdge = null;
    state.dirty = true;
    syncTopbarFromDraft();
    renderList();
    renderNodes();
    setStatus("New workflow draft", "ok");
  }

  async function saveWorkflow() {
    syncDraftFromTopbar();
    // flush inspector into selected task first
    flushInspector();
    const payload = {
      name: state.draft.name,
      description: state.draft.description,
      execution_mode: state.draft.execution_mode,
      tasks: state.draft.tasks,
      scope: el.scope.value || "project",
      layout: state.layout,
    };
    try {
      const res = await api("/api/save", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      state.dirty = false;
      state.currentName = res.name;
      await refreshList();
      setStatus("Saved to " + res.path, "ok");
    } catch (err) {
      setStatus(String(err.message || err), "err");
    }
  }

  async function deleteWorkflow() {
    const name = state.currentName || (el.name.value || "").trim().toLowerCase().replace(/\s+/g, "-");
    if (!name) {
      setStatus("No workflow selected to delete", "err");
      return;
    }
    const ok = window.confirm(`Delete workflow "${name}" from disk?\nThis cannot be undone.`);
    if (!ok) return;
    try {
      await api("/api/workflows/" + encodeURIComponent(name), { method: "DELETE" });
      state.currentName = null;
      state.dirty = false;
      await refreshList();
      if (state.workflows.length) {
        await loadWorkflow(state.workflows[0].name);
      } else {
        newWorkflow();
      }
      setStatus("Deleted workflow " + name, "ok");
    } catch (err) {
      setStatus(String(err.message || err), "err");
    }
  }

  function flushInspector() {
    if (!state.selectedId) return;
    const task = taskById(state.selectedId);
    if (!task) return;
    const oldId = task.id;
    let newId = (el.nodeId.value || "").trim().toLowerCase().replace(/\s+/g, "-");
    if (!newId) newId = oldId;
    if (newId !== oldId) {
      if (taskById(newId)) {
        setStatus("Duplicate task id: " + newId, "err");
        el.nodeId.value = oldId;
        return;
      }
      task.id = newId;
      if (state.layout.nodes[oldId]) {
        state.layout.nodes[newId] = state.layout.nodes[oldId];
        delete state.layout.nodes[oldId];
      }
      state.draft.tasks.forEach((t) => {
        t.depends_on = (t.depends_on || []).map((d) => (d === oldId ? newId : d));
      });
      state.selectedId = newId;
    }
    task.description = el.nodeDesc.value.trim();
    task.difficulty = el.nodeDiff.value;
    task.prompt = el.nodePrompt.value;
    task.allowed_tools = readChecked(el.toolsPanel);
    task.depends_on = readChecked(el.depsPanel);
    markDirty();
  }

  function escapeHtml(s) {
    return String(s)
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }
  function escapeAttr(s) {
    return escapeHtml(s).replaceAll("'", "&#39;");
  }
  function cssEscape(s) {
    if (window.CSS && CSS.escape) return CSS.escape(s);
    return String(s).replace(/"/g, '\\"');
  }

  function bind() {
    document.getElementById("btn-new").onclick = newWorkflow;
    document.getElementById("btn-refresh").onclick = async () => {
      await refreshList();
      setStatus("List refreshed", "ok");
    };
    document.getElementById("btn-add-node").onclick = addNode;
    document.getElementById("btn-auto-layout").onclick = autoLayout;
    document.getElementById("btn-delete-wf").onclick = deleteWorkflow;
    document.getElementById("btn-save").onclick = saveWorkflow;
    document.getElementById("btn-del-node").onclick = deleteSelected;

    el.toolsToggle.addEventListener("click", (e) => {
      e.stopPropagation();
      togglePanel(el.toolsPanel);
    });
    el.depsToggle.addEventListener("click", (e) => {
      e.stopPropagation();
      togglePanel(el.depsPanel);
    });
    el.toolsPanel.addEventListener("click", (e) => e.stopPropagation());
    el.depsPanel.addEventListener("click", (e) => e.stopPropagation());
    document.addEventListener("click", () => closeMultiSelects());

    ["wf-name", "wf-desc", "wf-mode", "wf-scope"].forEach((id) => {
      document.getElementById(id).addEventListener("input", () => {
        syncDraftFromTopbar();
        markDirty();
      });
      document.getElementById(id).addEventListener("change", () => {
        syncDraftFromTopbar();
        markDirty();
      });
    });

    ["node-id", "node-desc", "node-diff", "node-prompt"].forEach((id) => {
      const node = document.getElementById(id);
      node.addEventListener("change", () => {
        flushInspector();
        renderNodes();
      });
      if (node.tagName === "TEXTAREA" || node.type === "text") {
        node.addEventListener("blur", () => {
          flushInspector();
          renderNodes();
        });
      }
    });

    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
    window.addEventListener("keydown", (e) => {
      if (e.key === "Delete" || e.key === "Backspace") {
        const tag = (e.target && e.target.tagName) || "";
        if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
        e.preventDefault();
        deleteSelected();
      }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        saveWorkflow();
      }
    });

    el.wrap.addEventListener("mousedown", (e) => {
      if (e.button === 1 || (e.button === 0 && e.target === el.wrap)) {
        state.pan = {
          startX: e.clientX,
          startY: e.clientY,
          origX: state.view.x,
          origY: state.view.y,
        };
        state.selectedId = null;
        state.selectedEdge = null;
        renderNodes();
        e.preventDefault();
      }
    });
    el.wrap.addEventListener("wheel", (e) => {
      e.preventDefault();
      const delta = e.deltaY > 0 ? 0.9 : 1.1;
      const next = Math.min(2.5, Math.max(0.35, state.view.scale * delta));
      const rect = el.wrap.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      const wx = (mx - state.view.x) / state.view.scale;
      const wy = (my - state.view.y) / state.view.scale;
      state.view.scale = next;
      state.view.x = mx - wx * next;
      state.view.y = my - wy * next;
      applyView();
    }, { passive: false });
  }

  async function init() {
    bind();
    applyView();
    state.meta = await api("/api/meta");
    state.tools = state.meta.tools || [];
    el.metaFoot.textContent = `project: ${state.meta.project_dir || "-"}\nuser: ${state.meta.user_dir || "-"}`;
    await refreshList();
    if (state.workflows.length) {
      await loadWorkflow(state.workflows[0].name);
    } else {
      newWorkflow();
    }
  }

  init().catch((err) => setStatus(String(err.message || err), "err"));
})();
