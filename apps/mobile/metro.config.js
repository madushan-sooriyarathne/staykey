// Sentry's config adds debug ids to bundles, so production stack traces map back to the source.
const { getSentryExpoConfig } = require("@sentry/react-native/metro");

module.exports = getSentryExpoConfig(__dirname);
