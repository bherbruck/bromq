// TypeScript declarations for Monaco editor intellisense in BroMQ scripts
// These provide autocomplete for the JavaScript APIs available to scripts

export const SCRIPT_TYPE_DEFINITIONS = `
// Event object available in all scripts
declare const event: {
  /** Event type: 'publish', 'connect', 'disconnect', or 'subscribe' */
  type: 'publish' | 'connect' | 'disconnect' | 'subscribe';

  /** MQTT topic (for publish and subscribe events) */
  topic?: string;

  /** Message payload as string (for publish events) */
  payload?: string;

  /** MQTT Client ID */
  clientId: string;

  /** Authenticated username (may be empty for anonymous) */
  username?: string;

  /** Quality of Service level: 0, 1, or 2 (for publish and subscribe events) */
  qos?: number;

  /** Retain flag (for publish events) */
  retain?: boolean;

  /** Clean session flag (for connect events) */
  cleanSession?: boolean;

  /** Error message (for disconnect events with errors) */
  error?: string;
};

// Logging API
declare const log: {
  /**
   * Log a debug message
   * @param messages - Messages to log (will be joined with spaces)
   */
  debug(...messages: any[]): void;

  /**
   * Log an info message
   * @param messages - Messages to log (will be joined with spaces)
   */
  info(...messages: any[]): void;

  /**
   * Log a warning message
   * @param messages - Messages to log (will be joined with spaces)
   */
  warn(...messages: any[]): void;

  /**
   * Log an error message
   * @param messages - Messages to log (will be joined with spaces)
   */
  error(...messages: any[]): void;
};

// MQTT publish API
declare const mqtt: {
  /**
   * Publish a message to an MQTT topic
   * @param topic - MQTT topic to publish to
   * @param payload - Message payload (string)
   * @param qos - Quality of Service: 0 (at most once), 1 (at least once), or 2 (exactly once)
   * @param retain - Whether to retain the message on the broker
   */
  publish(topic: string, payload: string, qos: 0 | 1 | 2, retain: boolean): void;
};

// Script-scoped state API
declare const state: {
  /**
   * Store a value in script-scoped state
   * @param key - State key (unique within this script)
   * @param value - Value to store (any JSON-serializable type)
   * @param options - Optional settings
   * @param options.ttl - Time to live in seconds (auto-delete after this time)
   */
  set(key: string, value: any, options?: { ttl?: number }): void;

  /**
   * Retrieve a value from script-scoped state
   * @param key - State key
   * @returns The stored value, or undefined if not found or expired
   */
  get(key: string): any;

  /**
   * Delete a value from script-scoped state
   * @param key - State key to delete
   */
  delete(key: string): void;

  /**
   * List all keys in script-scoped state
   * @returns Array of all keys currently in state
   */
  keys(): string[];
};

// Global state API (shared across all scripts)
declare const global: {
  /**
   * Store a value in global state (shared across all scripts)
   * @param key - State key (unique globally)
   * @param value - Value to store (any JSON-serializable type)
   * @param options - Optional settings
   * @param options.ttl - Time to live in seconds (auto-delete after this time)
   */
  set(key: string, value: any, options?: { ttl?: number }): void;

  /**
   * Retrieve a value from global state
   * @param key - State key
   * @returns The stored value, or undefined if not found or expired
   */
  get(key: string): any;

  /**
   * Delete a value from global state
   * @param key - State key to delete
   */
  delete(key: string): void;

  /**
   * List all keys in global state
   * @returns Array of all keys currently in global state
   */
  keys(): string[];
};

// Built-in JavaScript globals that are available
declare const Date: DateConstructor;
declare const JSON: JSON;
declare const Math: Math;
declare const Number: NumberConstructor;
declare const String: StringConstructor;
declare const Array: ArrayConstructor;
declare const Object: ObjectConstructor;
`

export const SCRIPT_EXAMPLE_TEMPLATES = {
  'message-logger': `// Log all published messages
log.info('Message on', event.topic, ':', event.payload);
log.debug('From client:', event.clientId, 'User:', event.username);`,

  'temperature-alert': `// Monitor temperature and send alerts
const temp = parseFloat(event.payload);
const threshold = 30.0;
const sensorId = event.topic.split('/')[1];

if (temp > threshold) {
  const alertKey = 'last_alert:' + sensorId;
  const lastAlert = state.get(alertKey);
  const now = Date.now();

  // Only alert once per 5 minutes
  if (!lastAlert || (now - lastAlert) > 300000) {
    log.warn('HIGH TEMPERATURE:', sensorId, temp, '°C');

    mqtt.publish('alerts/temperature', JSON.stringify({
      sensor_id: sensorId,
      temperature: temp,
      threshold: threshold,
      timestamp: now
    }), 1, false);

    state.set(alertKey, now, {ttl: 300});
  }
}`,

  'connection-tracker': `// Track client connections
if (event.type === 'connect') {
  state.set('connect_time:' + event.clientId, Date.now());
  log.info('Client connected:', event.clientId);

  // Increment global connection counter
  const total = global.get('total_connections') || 0;
  global.set('total_connections', total + 1);

} else if (event.type === 'disconnect') {
  const connectTime = state.get('connect_time:' + event.clientId);

  if (connectTime) {
    const duration = Math.floor((Date.now() - connectTime) / 1000);
    log.info('Client disconnected:', event.clientId, 'Duration:', duration, 'seconds');
    state.delete('connect_time:' + event.clientId);
  }
}`,

  'rate-limiter': `// Rate limit messages per client
const countKey = 'msg_count:' + event.clientId;
const count = state.get(countKey) || 0;
const maxPerMinute = 100;

if (count >= maxPerMinute) {
  log.warn('Rate limit exceeded:', event.clientId);
} else if ((count + 1) % 10 === 0) {
  log.debug('Client', event.clientId, 'sent', count + 1, 'messages this minute');
}

// Increment with 60 second TTL
state.set(countKey, count + 1, {ttl: 60});`,

  blank: `// Your script code here
// Available APIs: event, log, mqtt, state, global

log.info('Script triggered:', event.type);
`,
}
