# ============================================================
# Clawd-Crawlee: Headless browser scraping service
# Gebaseerd op Crawlee (Playwright) met een eenvoudige HTTP API
# ============================================================
FROM node:22-bookworm-slim

# Playwright system dependencies + fonts voor CJK pagina's
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    fonts-liberation \
    fonts-noto-color-emoji \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Package files kopieren en dependencies installeren
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev

# Playwright browsers installeren (alleen Chromium om ruimte te besparen)
ENV PLAYWRIGHT_BROWSERS_PATH=/app/browsers
RUN npx playwright install --with-deps chromium

# Applicatie kopieren
COPY . .

# Non-root user
RUN groupadd -g 1001 crawlee && \
    useradd -u 1001 -g crawlee -m crawlee && \
    chown -R crawlee:crawlee /app
USER crawlee

# Storage directory voor Crawlee
RUN mkdir -p /app/storage

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:3000/health || exit 1

CMD ["node", "server.js"]
