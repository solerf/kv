const fs = require("fs");
const path = require("path");

const dest = path.join(__dirname, "..", "static");

const assets = [
  {
    src: "node_modules/bootstrap/dist/css/bootstrap.min.css",
    out: "css/bootstrap.min.css",
  },
  {
    src: "node_modules/neobrutalismcss/dist/css/neobrutalismcss.css",
    out: "css/neobrutalismcss.css",
  },
  {
    src: "node_modules/bootstrap/dist/js/bootstrap.bundle.min.js",
    out: "js/bootstrap.bundle.min.js",
  },
  {
    src: "node_modules/jquery/dist/jquery.min.js",
    out: "js/jquery.min.js",
  },
  {
    src: "node_modules/chart.js/dist/chart.umd.js",
    out: "js/chart.umd.js",
  },
];

for (const { src, out } of assets) {
  const srcPath = path.join(__dirname, "..", src);
  const outPath = path.join(dest, out);

  fs.mkdirSync(path.dirname(outPath), { recursive: true });

  if (!fs.existsSync(srcPath)) {
    console.warn(`WARN: ${src} not found, skipping`);
    continue;
  }

  fs.copyFileSync(srcPath, outPath);
  console.log(`${src} -> static/${out}`);
}
