const fs = require("fs");
const os = require("os");
const path = require("path");

function parseDefaultOrder(text) {
  const match = text.match(/DEFAULT_MODEL_ORDER=\[([^\]]+)\]/);
  if (!match) {
    return [];
  }
  const items = match[1]
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
  return items.length ? items : [];
}

function findCodexMaxVersions(text) {
  const versions = new Set();
  const matches = text.match(/gpt-[0-9](?:\.[0-9]+)?-codex-max/g) || [];
  matches.forEach((item) => versions.add(item));

  function key(value) {
    const parts = value.split("-")[1].split(".");
    return parts.map((part) => parseInt(part, 10));
  }

  return Array.from(versions).sort((a, b) => {
    const ka = key(a);
    const kb = key(b);
    const len = Math.max(ka.length, kb.length);
    for (let i = 0; i < len; i += 1) {
      if (ka[i] === undefined) return -1;
      if (kb[i] === undefined) return 1;
      if (ka[i] !== kb[i]) return kb[i] - ka[i];
    }
    return 0;
  });
}

function findGpt5Models(text) {
  const models = new Set();
  const matches = text.match(/gpt-5[\w\.-]*/g) || [];
  matches.forEach((item) => models.add(item));
  return Array.from(models).sort();
}

function stripQuotes(name) {
  return name.trim().replace(/^['"]|['"]$/g, "");
}

function normalizeName(name) {
  const raw = stripQuotes(name);
  const match = raw.match(/^(gpt-5)-([0-9]+)([\w\.-]*)$/);
  if (!match) {
    return raw;
  }
  return `${match[1]}.${match[2]}${match[3]}`;
}

function quote(name) {
  return `"${normalizeName(name)}"`;
}

function versionTuple(name) {
  const match = name.match(/gpt-5[.-]([0-9]+(?:\.[0-9]+)?)/);
  if (!match) {
    return [0];
  }
  return match[1]
    .replace(/-/g, ".")
    .split(".")
    .map((part) => parseInt(part, 10));
}

function modelSortKey(name) {
  const version = versionTuple(name).map((num) => -num);
  let category = 2;
  if (name.includes("codex-max")) {
    category = 0;
  } else if (name.includes("codex") && !name.includes("mini")) {
    category = 1;
  } else if (name.includes("codex-mini")) {
    category = 3;
  }
  return { version, category, name };
}

function compareTuples(left, right) {
  const len = Math.max(left.length, right.length);
  for (let i = 0; i < len; i += 1) {
    if (left[i] === undefined) return -1;
    if (right[i] === undefined) return 1;
    if (left[i] !== right[i]) return left[i] - right[i];
  }
  return left.length - right.length;
}

function orderModels(models) {
  const normalized = new Set();
  for (const model of models) {
    if (stripQuotes(model)) {
      normalized.add(normalizeName(model));
    }
  }
  const ordered = Array.from(normalized).sort((a, b) => {
    const ka = modelSortKey(a);
    const kb = modelSortKey(b);
    const cmpVersion = compareTuples(ka.version, kb.version);
    if (cmpVersion !== 0) return cmpVersion;
    if (ka.category !== kb.category) return ka.category - kb.category;
    return ka.name.localeCompare(kb.name);
  });
  return ordered.map((model) => quote(model));
}

function buildApikeyList(text, includeMini) {
  const defaultOrder = parseDefaultOrder(text).map((item) => stripQuotes(item));
  const codexVersions = findCodexMaxVersions(text);
  const gpt5Models = findGpt5Models(text);

  let candidates = new Set([...gpt5Models, ...defaultOrder, ...codexVersions]);
  if (candidates.size === 0) {
    candidates = new Set(["gpt-5.1-codex-max"]);
  }

  if (!includeMini) {
    candidates = new Set(
      Array.from(candidates).filter((model) => !model.toLowerCase().includes("mini"))
    );
  }

  return orderModels(candidates);
}

function replaceAuthMethodArray(text, field, newItems) {
  const newArray = `[${newItems.join(",")}]`;
  const newField = `${field}:${newArray}`;

  const patternArray = new RegExp(`${field}:\\s*\\[[^\\]]*\\]`, "s");
  if (patternArray.test(text)) {
    return [text.replace(patternArray, newField), true];
  }

  const patternVar = new RegExp(`${field}:[A-Z][A-Z0-9_]*`);
  if (patternVar.test(text)) {
    return [text.replace(patternVar, newField), true];
  }

  return [text, false];
}

function ensureApikey(text, includeMini) {
  const newList = buildApikeyList(text, includeMini);
  return replaceAuthMethodArray(text, "apikey", newList);
}

function ensureChatgpt(text, includeMini) {
  const newList = buildApikeyList(text, includeMini);
  return replaceAuthMethodArray(text, "chatgpt", newList);
}

function removeAuthOnly(text) {
  const pattern = /CHAT_GPT_AUTH_ONLY_MODELS=new Set\(\[([^\]]*?)\]\)/;
  const match = pattern.exec(text);
  if (!match) {
    return [text, false];
  }
  if (!match[1].trim()) {
    return [text, false];
  }
  const replacement = "CHAT_GPT_AUTH_ONLY_MODELS=new Set([])";
  return [text.replace(pattern, replacement), true];
}

function patchFile(filePath, includeMini) {
  const backupPath = `${filePath}.bak`;
  if (!fs.existsSync(backupPath)) {
    fs.copyFileSync(filePath, backupPath);
    console.log(`[backup]  ${backupPath}`);
  }

  const original = fs.readFileSync(filePath, "utf8");
  let text = original;
  let changedApikey = false;
  let changedChatgpt = false;
  let changedAuth = false;

  [text, changedApikey] = ensureApikey(text, includeMini);
  [text, changedChatgpt] = ensureChatgpt(text, includeMini);
  [text, changedAuth] = removeAuthOnly(text);

  if (changedApikey || changedChatgpt || changedAuth) {
    fs.writeFileSync(filePath, text, "utf8");
    const changes = [];
    if (changedApikey) changes.push("apikey");
    if (changedChatgpt) changes.push("chatgpt");
    if (changedAuth) changes.push("auth_only");
    console.log(`[patched] ${filePath} (${changes.join(", ")})`);
  } else {
    console.log(`[skip]    ${filePath} (already compliant)`);
  }
}

function autoDiscover() {
  const home = os.homedir();
  const roots = [path.join(home, ".vscode", "extensions")];
  if (process.platform === "win32") {
    const userProfile = process.env.USERPROFILE || home;
    roots.push(path.join(userProfile, ".vscode", "extensions"));
  }

  const found = [];
  for (const root of roots) {
    const parent = root;
    if (!fs.existsSync(parent)) {
      continue;
    }
    const entries = fs.readdirSync(parent, { withFileTypes: true });
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      if (!entry.name.startsWith("openai.chatgpt")) continue;
      const webview = path.join(parent, entry.name, "webview", "assets");
      if (!fs.existsSync(webview) || !fs.statSync(webview).isDirectory()) {
        continue;
      }
      const assets = fs.readdirSync(webview);
      for (const asset of assets) {
        if (/^index-.*\.js$/.test(asset)) {
          found.push(path.join(webview, asset));
        }
      }
    }
  }
  return found;
}

function autoDiscoverBaks() {
  const home = os.homedir();
  const roots = [path.join(home, ".vscode", "extensions")];
  if (process.platform === "win32") {
    const userProfile = process.env.USERPROFILE || home;
    roots.push(path.join(userProfile, ".vscode", "extensions"));
  }

  const found = [];
  for (const root of roots) {
    const parent = root;
    if (!fs.existsSync(parent)) {
      continue;
    }
    const entries = fs.readdirSync(parent, { withFileTypes: true });
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      if (!entry.name.startsWith("openai.chatgpt")) continue;
      const webview = path.join(parent, entry.name, "webview", "assets");
      if (!fs.existsSync(webview) || !fs.statSync(webview).isDirectory()) {
        continue;
      }
      const assets = fs.readdirSync(webview);
      for (const asset of assets) {
        if (/^index-.*\.js\.bak$/.test(asset)) {
          found.push(path.join(webview, asset));
        }
      }
    }
  }
  return found;
}

function restore(bakFiles) {
  const targets = bakFiles.length ? bakFiles : autoDiscoverBaks();
  if (targets.length === 0) {
    console.log("没有找到可恢复的 .bak 文件。");
    return 1;
  }
  for (const bakPath of targets) {
    if (!fs.existsSync(bakPath)) {
      console.log(`[error]   ${bakPath} does not exist`);
      continue;
    }
    const original = bakPath.replace(/\.bak$/, "");
    fs.copyFileSync(bakPath, original);
    console.log(`[restored] ${original} <- ${bakPath}`);
  }
  console.log("提示：如仍异常，建议重新安装插件或手动替换原文件。");
  return 0;
}

function main(argv) {
  const args = argv.slice(2);
  const files = [];
  let auto = false;
  let restoreFlag = false;
  let includeMini = false;

  for (const arg of args) {
    if (arg === "--auto") {
      auto = true;
    } else if (arg === "--restore") {
      restoreFlag = true;
    } else if (arg === "--include-mini") {
      includeMini = true;
    } else {
      files.push(arg);
    }
  }

  if (restoreFlag) {
    return restore(files);
  }

  const targets = [];
  if (files.length) {
    targets.push(...files);
  }
  if (auto) {
    targets.push(...autoDiscover());
  }

  if (targets.length === 0) {
    console.log("没有找到需要 patch 的文件。请指定文件或使用 --auto。");
    return 1;
  }

  for (const target of targets) {
    if (!fs.existsSync(target)) {
      console.log(`[error]   ${target} does not exist`);
      continue;
    }
    patchFile(target, includeMini);
  }

  console.log("操作完成。请重启 VS Code 插件以加载新资源。");
  return 0;
}

process.exitCode = main(process.argv);
