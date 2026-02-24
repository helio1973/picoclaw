# ============================================================
# Clawd-Crawlee: Headless browser scraping service
# Uses Playwright's official minimal image with Chromium pre-installed.
# ============================================================
FROM mcr.microsoft.com/playwright:v1.50.0-noble

# curl voor healthcheck, fonts voor CJK pagina's
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    fonts-noto-color-emoji \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Dependencies installeren en cache opruimen in één layer
COPY package.json ./
RUN npm install --omit=dev && npm cache clean --force

# Applicatie kopieren
COPY . .

# Non-root user
RUN groupadd -g 1001 crawlee && \
    useradd -u 1001 -g crawlee -m crawlee && \
    chown -R crawlee:crawlee /app
USER crawlee

RUN mkdir -p /app/storage

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:3000/health || exit 1

CMD ["node", "server.js"]
