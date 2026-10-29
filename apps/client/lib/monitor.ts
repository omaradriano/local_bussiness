import * as Sentry from '@sentry/react-native';

// Literal process.env.EXPO_PUBLIC_* reads so Expo inlines them at build time.
const dsn = process.env.EXPO_PUBLIC_SENTRY_DSN;

/**
 * Error reporting (Sentry), only in builds with EXPO_PUBLIC_SENTRY_DSN. Nothing personal leaves:
 * no user, no request bodies or query strings, no breadcrumbs with typed text.
 */
export function initMonitoring() {
  if (!dsn || __DEV__) return;
  Sentry.init({
    dsn,
    environment: process.env.EXPO_PUBLIC_APP_ENV ?? 'staging',
    sendDefaultPii: false,
    beforeSend(event) {
      delete event.user;
      if (event.request) {
        delete event.request.data;
        delete event.request.query_string;
        delete event.request.cookies;
      }
      return event;
    },
    beforeBreadcrumb: (crumb) => (crumb.category === 'ui.input' ? null : crumb),
  });
}

export const withMonitoring = Sentry.wrap;
