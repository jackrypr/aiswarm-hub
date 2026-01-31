# AI Swarm Prediction Hub - Frontend
# For Railway: deploy as separate service pointing to /frontend

FROM node:22-alpine AS builder

WORKDIR /app

# Copy package files for caching
COPY package.json package-lock.json ./
RUN npm ci

# Copy source and build
COPY . .
RUN npm run build

# Production - serve with lightweight server
FROM node:22-alpine

WORKDIR /app

# Install serve for static file serving
RUN npm install -g serve

# Copy built files
COPY --from=builder /app/dist ./dist

# Railway injects PORT
ENV PORT=3000
EXPOSE 3000

# Serve the built app
CMD ["serve", "-s", "dist", "-l", "3000"]
