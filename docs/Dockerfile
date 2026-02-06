# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm install && npm cache clean --force

# Copy source code
COPY . .

# Build the documentation
RUN npm run docs:build

# Production stage - serve with nginx
FROM nginx:alpine

# Copy nginx config and entrypoint
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Copy built documentation from builder to nginx html directory
COPY --from=builder /app/.vitepress/dist /usr/share/nginx/html

# Set default PORT (Railway will override)
ENV PORT=3000

# Expose port
EXPOSE 3000

# Start nginx via entrypoint (substitutes $PORT in nginx config)
ENTRYPOINT ["/docker-entrypoint.sh"]
