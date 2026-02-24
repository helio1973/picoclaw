---
name: deep-scraper
description: "Deep web scraping via the clawd-crawlee service. Scrape single pages or crawl entire sites with a headless browser. Handles JavaScript-rendered content, SPAs, and dynamic pages."
---

# Deep Scraper Skill

Use the clawd-crawlee service to scrape web pages rendered by a full headless browser (Chromium). This is useful for JavaScript-heavy sites, SPAs, and content behind client-side rendering that simple HTTP fetches cannot handle.

The service is available at `${CRAWLEE_BASE_URL}` (default: `http://clawd-crawlee:3000`).

## Single Page Scrape

Scrape a single page and extract its content:

```bash
curl -s -X POST "${CRAWLEE_BASE_URL:-http://clawd-crawlee:3000}/scrape" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'
```

With a CSS selector to target specific content:

```bash
curl -s -X POST "${CRAWLEE_BASE_URL:-http://clawd-crawlee:3000}/scrape" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/docs", "selector": "article.main-content"}'
```

Wait for a specific element before extracting (useful for SPAs):

```bash
curl -s -X POST "${CRAWLEE_BASE_URL:-http://clawd-crawlee:3000}/scrape" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/app", "waitFor": "#content-loaded"}'
```

**Response:**
```json
{
  "url": "https://example.com",
  "title": "Page Title",
  "content": "Extracted text content...",
  "links": [{"text": "Link text", "href": "https://..."}]
}
```

## Deep Scrape (Multi-page Crawl)

Crawl a site by following links from the starting URL:

```bash
curl -s -X POST "${CRAWLEE_BASE_URL:-http://clawd-crawlee:3000}/deep-scrape" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://docs.example.com", "maxDepth": 2, "maxPages": 10}'
```

**Parameters:**
- `url` (required): Starting URL
- `maxDepth` (optional, default 2): How many link levels deep to follow
- `maxPages` (optional, default 20): Maximum number of pages to scrape
- `selector` (optional): CSS selector to extract specific content from each page

**Response:**
```json
{
  "pages": [
    {"url": "https://...", "title": "...", "content": "...", "depth": 0},
    {"url": "https://...", "title": "...", "content": "...", "depth": 1}
  ],
  "total": 2
}
```

## Configuration

Add `clawd-crawlee` to the SSRF allowlist in `config.json` so the agent can reach the service on the Docker network:

```json
{
  "tools": {
    "web": {
      "allowed_hosts": ["clawd-crawlee"]
    }
  }
}
```

## Tips

- Use `/scrape` for quick single-page content extraction
- Use `/deep-scrape` for documentation sites, sitemaps, or research tasks
- Set `maxPages` conservatively to avoid long-running crawls
- Use `selector` to focus on relevant content and reduce noise
- The service stays on the same domain by default (no cross-domain crawling)
- Content is truncated to 50KB per page (deep) or 100KB (single) to keep responses manageable
