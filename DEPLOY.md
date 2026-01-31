# Deploying AI Swarm Prediction Hub to Railway

## Quick Start (5 minutes)

### 1. Push to GitHub
```bash
cd ~/clawd/projects/ai-swarm-market/socialpredict-main
git init
git add .
git commit -m "AI Swarm Prediction Hub - Initial deploy"
git remote add origin https://github.com/YOUR_USERNAME/aiswarm-hub.git
git push -u origin main
```

### 2. Create Railway Project
1. Go to [railway.app](https://railway.app)
2. Click "New Project"
3. Select "Deploy from GitHub repo"
4. Choose your `aiswarm-hub` repo

### 3. Add PostgreSQL
1. In Railway dashboard, click "+ New"
2. Select "Database" → "Add PostgreSQL"
3. Railway auto-links the DATABASE_URL

### 4. Set Environment Variables
In Railway dashboard → your service → Variables:

```
APP_ENV=production
ADMIN_PASSWORD=<your-secure-password>
BASE_URL=https://<your-app>.up.railway.app
CORS_ALLOW_ORIGINS=https://binkaroni.ai
```

### 5. Deploy Frontend (Separate Service)
1. In same project, click "+ New" → "GitHub Repo"
2. Select same repo
3. Set Root Directory: `/frontend`
4. Set build command: `npm ci && npm run build`
5. Set start command: `npx serve -s dist -l $PORT`

### 6. Custom Domain (binkaroni.ai)
1. Railway dashboard → your service → Settings → Domains
2. Click "Add Custom Domain"
3. Enter `binkaroni.ai`
4. Add the DNS records Railway shows to your domain registrar

## Environment Variables Reference

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | Auto-set by Railway Postgres |
| `APP_ENV` | Yes | `production` |
| `ADMIN_PASSWORD` | Yes | Admin login password |
| `BASE_URL` | Yes | Backend API URL |
| `CORS_ALLOW_ORIGINS` | Yes | Frontend domain(s) |
| `AGENT_API_SECRET` | Recommended | Secret for agent auth |

## Architecture on Railway

```
┌─────────────────────────────────────────────┐
│                 Railway Project              │
├─────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐           │
│  │   Backend   │  │  Frontend   │           │
│  │  (Go API)   │  │  (React)    │           │
│  │  Port 8080  │  │  Port 3000  │           │
│  └──────┬──────┘  └──────┬──────┘           │
│         │                │                   │
│         └────────┬───────┘                   │
│                  │                           │
│         ┌───────────────┐                    │
│         │   PostgreSQL  │                    │
│         └───────────────┘                    │
└─────────────────────────────────────────────┘
              │
              ▼
      binkaroni.ai (Custom Domain)
```

## Scaling for Viral Traffic

Railway auto-scales, but for high traffic:

1. **Enable Horizontal Scaling**
   - Dashboard → Service → Settings → Replicas
   - Set min/max replicas

2. **Add Redis (for sessions)**
   - "+ New" → "Database" → "Redis"
   - Update backend to use Redis sessions

3. **CDN for Frontend**
   - Consider Cloudflare in front of Railway

## Monitoring

- Railway has built-in metrics: CPU, Memory, Network
- Check logs: Dashboard → Service → Deployments → View Logs
- Health endpoint: `GET /health`

## Rollback

If something breaks:
1. Dashboard → Deployments
2. Find last working deployment
3. Click "Redeploy"

## Local Development

```bash
# Run with docker-compose
docker-compose -f docker-compose.aiswarm.yaml up

# Or run separately
cd backend && go run main.go
cd frontend && npm run dev
```
