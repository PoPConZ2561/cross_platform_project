import tailwindcss from "@tailwindcss/vite";

function forceHmrLocalhost() {
  return {
    name: "wails-force-hmr-localhost",
    apply: "serve" as const,
    configureServer(server: any) {
      const Buf: any = (globalThis as any).Buffer;
      server.middlewares.stack.unshift({
        route: "",
        handle(req: any, res: any, next: any) {
          const url: string = req.url || "";
          if (!url.includes("@vite/client")) return next();
          delete req.headers["if-none-match"];
          delete req.headers["if-modified-since"];
          const origEnd = res.end.bind(res);
          const chunks: any[] = [];
          res.write = function (chunk: any, ...rest: any[]) {
            if (chunk) chunks.push(Buf.from(chunk));
            const cb = rest.find((a) => typeof a === "function");
            if (cb) cb();
            return true;
          };
          res.end = function (chunk: any, ...rest: any[]) {
            if (typeof chunk === "function") chunk = undefined;
            if (chunk) chunks.push(Buf.from(chunk));
            let body: string = Buf.concat(chunks).toString("utf8");
            body = body.replace(
              "null || importMetaUrl.hostname",
              '"localhost"',
            );
            res.setHeader("content-length", Buf.byteLength(body));
            const cb = rest.find((a) => typeof a === "function");
            return origEnd(body, cb);
          };
          next();
        },
      });
    },
  };
}

export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  devtools: { enabled: true },
  ssr: false,
  css: ["~/assets/css/main.css"],
  $production: {
    nitro: {
      output: {
        publicDir: "../frontend/dist",
      },
    },
  },
  router: {
    options: {
      hashMode: true,
    },
  },
  vite: {
    plugins: [forceHmrLocalhost(), tailwindcss() as any],
  },
});