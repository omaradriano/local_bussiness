import { defineRailway, preserve, project, service } from "railway/iac";

// Code is uploaded with `railway up services/api --path-as-root`, so paths are relative to services/api.
// Both services run the same image; only the start command differs.
const build = { builder: "DOCKERFILE", dockerfilePath: "Dockerfile" } as const;
const replicas = { "us-west2": 1 }; // next to Supabase (us-west-2)

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
      DATABASE_URL: preserve(), // secret, set by the owner in Railway; never in the repo
    },
  });

  const worker = service("worker", {
    build,
    replicas,
    deploy: { startCommand: "worker" },
  });

  return project("local-bussiness", {
    resources: [api, worker],
  });
});
