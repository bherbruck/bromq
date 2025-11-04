// Temperature Alert
// Monitors temperature readings and sends alerts when threshold exceeded
// Uses state to prmsg alert flooding

const temp = parseFloat(msg.payload);

if (isNaN(temp)) {
    log.warn('Invalid temperature value:', msg.payload);
    return;
}

const threshold = 30.0;
const sensorId = msg.topic.split('/')[1]; // Extract from sensors/{id}/temperature

if (temp > threshold) {
    // Check if we already alerted recently (rate limiting)
    const alertKey = 'last_alert:' + sensorId;
    const lastAlert = state.get(alertKey);
    const now = Date.now();

    // Only alert once per 5 minutes per sensor
    if (!lastAlert || (now - lastAlert) > 300000) {
        log.warn('HIGH TEMPERATURE ALERT:', sensorId, '-', temp, '°C');

        // Publish alert
        mqtt.publish('alerts/temperature', JSON.stringify({
            sensor_id: sensorId,
            temperature: temp,
            threshold: threshold,
            timestamp: now,
            client_id: msg.clientId
        }), 1, false);

        // Update last alert time with 5 minute TTL
        state.set(alertKey, now, {ttl: 300});
    }
}

// Track temperature history (last 10 readings per sensor)
const historyKey = 'history:' + sensorId;
const history = state.get(historyKey) || [];
history.push({temp: temp, time: Date.now()});

// Keep only last 10 readings
if (history.length > 10) {
    history.shift();
}

state.set(historyKey, history);
