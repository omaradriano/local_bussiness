import { defineRailway, preserve, project, service } from "railway/iac";

// Code is uploaded with `railway up services/api --path-as-root`, so paths are relative to services/api.
// Both services run the same image; only the start command differs.
const build = { builder: "DOCKERFILE", dockerfilePath: "Dockerfile" } as const;
const replicas = { "us-west2": 1 }; // next to Supabase (us-west-2)
const photos = {
  AWS_REGION: "us-west-2",
  S3_BUCKET_PREFIX: "local-bussiness-staging",
  CATALOG_CDN_URL: "https://d1tklylecl3ocy.cloudfront.net",
};

export default defineRailway(() => {
  const api = service("api", {
    build,
    replicas,
    deploy: { startCommand: "api" },
    healthcheck: "/healthz",
    healthcheckTimeout: 30,
    env: {
      SUPABASE_URL: "https://upsolqbrrpvhiusnlsfh.supabase.co",
      CORS_ORIGINS: [
        "https://local-bussiness-staging.vercel.app",
        "https://local-bussiness-staging-*-omaradrianos-projects.vercel.app", // PR previews
        "http://localhost:8082",
      ].join(","),
      DATABASE_URL: preserve(), // secrets, set by the owner in Railway; never in the repo
      AWS_ACCESS_KEY_ID: preserve(),
      AWS_SECRET_ACCESS_KEY: preserve(),
      ...photos,
    },
  });

  // The worker reads the same secrets by reference to the api service (no copies).
  const worker = service("worker", {
    build,
    replicas,
    deploy: { startCommand: "worker" },
    env: {
      DATABASE_URL: api.env.DATABASE_URL,
      AWS_ACCESS_KEY_ID: api.env.AWS_ACCESS_KEY_ID,
      AWS_SECRET_ACCESS_KEY: api.env.AWS_SECRET_ACCESS_KEY,
      ...photos,
    },
  });

  return project("local-bussiness", {
    resources: [api, worker],
  });
});
