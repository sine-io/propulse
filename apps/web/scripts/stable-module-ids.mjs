import { createHash } from "node:crypto";

const pluginName = "StableModuleIdsPlugin";

export class StableModuleIdsPlugin {
  apply(compiler) {
    compiler.hooks.compilation.tap(pluginName, (compilation) => {
      compilation.hooks.moduleIds.tap(pluginName, (modules) => {
        const chunkGraph = compilation.chunkGraph;
        const moduleList = Array.from(modules);
        const usedIds = new Set(
          moduleList
            .map((module) => chunkGraph.getModuleId(module))
            .filter((id) => id !== null)
            .map(String),
        );
        const pending = moduleList
          .filter((module) => chunkGraph.getModuleId(module) === null)
          .map((module) => ({
            key: [
              stableModuleIdentifier(module.identifier(), compiler.context),
              module.type ?? "",
              module.layer ?? "",
            ].join("\0"),
            module,
          }))
          .sort((left, right) => left.key < right.key ? -1 : left.key > right.key ? 1 : 0);

        for (const { key, module } of pending) {
          const digest = createHash("sha256").update(key).digest("hex");
          let length = 10;
          let id = `m${digest.slice(0, length)}`;
          while (usedIds.has(id)) {
            length += 2;
            id = `m${digest.slice(0, length)}`;
          }
          usedIds.add(id);
          chunkGraph.setModuleId(module, id);
        }
      });
    });
  }
}

export function stableModuleIdentifier(identifier, projectRoot) {
  const root = normalizeSlashes(projectRoot).replace(/\/$/, "");
  const encodedRoot = encodeURIComponent(root);
  return normalizeSlashes(identifier)
    .replaceAll(encodedRoot, "%3Cproject-root%3E")
    .replaceAll(root, "<project-root>");
}

function normalizeSlashes(value) {
  return value.replaceAll("\\", "/");
}
