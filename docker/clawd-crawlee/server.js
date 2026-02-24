const http = require("node:http");
const { PlaywrightCrawler, Dataset } = require("crawlee");

const PORT = 3000;
const MAX_CONCURRENCY = parseInt(process.env.MAX_CONCURRENCY || "5", 10);
const REQUEST_TIMEOUT_MS = parseInt(
  process.env.REQUEST_TIMEOUT_MS || "60000",
  10
);
const LOG_LEVEL = process.env.LOG_LEVEL || "info";

// ─────────────────────────────────────────────
// Deep scrape: crawl een URL en volg links
// ─────────────────────────────────────────────
async function deepScrape({ url, maxDepth = 2, maxPages = 20, selector }) {
  const results = [];

  const crawler = new PlaywrightCrawler({
    maxConcurrency: MAX_CONCURRENCY,
    requestHandlerTimeoutSecs: REQUEST_TIMEOUT_MS / 1000,
    maxRequestsPerCrawl: maxPages,
    headless: true,
    launchContext: {
      launchOptions: {
        args: ["--no-sandbox", "--disable-setuid-sandbox"],
      },
    },
    async requestHandler({ request, page, enqueueLinks, log }) {
      const depth = request.userData.depth || 0;
      log.info(`Scraping [depth=${depth}]: ${request.url}`);

      // Wacht tot pagina geladen is
      await page.waitForLoadState("domcontentloaded");

      const title = await page.title();

      let content;
      if (selector) {
        const el = await page.$(selector);
        content = el ? await el.innerText() : await page.innerText("body");
      } else {
        // Probeer main content te vinden, fallback naar body
        const mainEl =
          (await page.$("main")) ||
          (await page.$("article")) ||
          (await page.$('[role="main"]'));
        content = mainEl
          ? await mainEl.innerText()
          : await page.innerText("body");
      }

      results.push({
        url: request.url,
        title,
        content: content.slice(0, 50000), // limiet per pagina
        depth,
      });

      // Enqueue links als we nog niet op max diepte zijn
      if (depth < maxDepth) {
        await enqueueLinks({
          userData: { depth: depth + 1 },
          // Blijf op hetzelfde domein
          strategy: "same-domain",
        });
      }
    },
    failedRequestHandler({ request, log }) {
      log.warning(`Request failed: ${request.url}`);
    },
  });

  await crawler.run([{ url, userData: { depth: 0 } }]);

  return results;
}

// ─────────────────────────────────────────────
// Enkele pagina scrape
// ─────────────────────────────────────────────
async function scrape({ url, selector, waitFor }) {
  let result = {};

  const crawler = new PlaywrightCrawler({
    maxConcurrency: 1,
    maxRequestsPerCrawl: 1,
    requestHandlerTimeoutSecs: REQUEST_TIMEOUT_MS / 1000,
    headless: true,
    launchContext: {
      launchOptions: {
        args: ["--no-sandbox", "--disable-setuid-sandbox"],
      },
    },
    async requestHandler({ page }) {
      await page.waitForLoadState("domcontentloaded");

      if (waitFor) {
        await page
          .waitForSelector(waitFor, { timeout: 10000 })
          .catch(() => {});
      }

      const title = await page.title();

      let content;
      if (selector) {
        const el = await page.$(selector);
        content = el ? await el.innerText() : "";
      } else {
        const mainEl =
          (await page.$("main")) ||
          (await page.$("article")) ||
          (await page.$('[role="main"]'));
        content = mainEl
          ? await mainEl.innerText()
          : await page.innerText("body");
      }

      const links = await page.$$eval("a[href]", (anchors) =>
        anchors
          .map((a) => ({ text: a.innerText.trim(), href: a.href }))
          .filter((l) => l.text && l.href.startsWith("http"))
          .slice(0, 100)
      );

      result = {
        url: page.url(),
        title,
        content: content.slice(0, 100000),
        links,
      };
    },
  });

  await crawler.run([url]);
  return result;
}

// ─────────────────────────────────────────────
// HTTP server
// ─────────────────────────────────────────────
function parseBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => {
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString()));
      } catch (e) {
        reject(new Error("Invalid JSON"));
      }
    });
    req.on("error", reject);
  });
}

function sendJSON(res, status, data) {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(data));
}

const server = http.createServer(async (req, res) => {
  // Health check
  if (req.url === "/health" && req.method === "GET") {
    return sendJSON(res, 200, { status: "ok" });
  }

  // Enkele pagina scrape
  if (req.url === "/scrape" && req.method === "POST") {
    try {
      const body = await parseBody(req);
      if (!body.url) return sendJSON(res, 400, { error: "url is required" });
      const result = await scrape(body);
      return sendJSON(res, 200, result);
    } catch (e) {
      return sendJSON(res, 500, { error: e.message });
    }
  }

  // Deep scrape (meerdere pagina's)
  if (req.url === "/deep-scrape" && req.method === "POST") {
    try {
      const body = await parseBody(req);
      if (!body.url) return sendJSON(res, 400, { error: "url is required" });
      const results = await deepScrape(body);
      return sendJSON(res, 200, { pages: results, total: results.length });
    } catch (e) {
      return sendJSON(res, 500, { error: e.message });
    }
  }

  sendJSON(res, 404, { error: "not found" });
});

server.listen(PORT, () => {
  console.log(`clawd-crawlee listening on port ${PORT}`);
  console.log(`  POST /scrape       - Single page scrape`);
  console.log(`  POST /deep-scrape  - Deep crawl with link following`);
  console.log(`  GET  /health       - Health check`);
  console.log(
    `  Config: maxConcurrency=${MAX_CONCURRENCY}, timeout=${REQUEST_TIMEOUT_MS}ms`
  );
});
