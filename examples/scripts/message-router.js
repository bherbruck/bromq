// Message Router
// Transforms and republishes messages to different topics

// Route legacy format to new format
if (msg.topic.startsWith('legacy/')) {
    try {
        // Parse legacy format (CSV: sensor_id,value,unit)
        const parts = msg.payload.split(',');
        const data = {
            sensor_id: parts[0],
            value: parseFloat(parts[1]),
            unit: parts[2],
            timestamp: Date.now()
        };

        // Republish in new JSON format
        const newTopic = 'v2/' + msg.topic.replace('legacy/', '');
        mqtt.publish(newTopic, JSON.stringify(data), msg.qos, false);

        log.debug('Transformed message:', msg.topic, '->', newTopic);
    } catch (e) {
        log.error('Failed to transform message:', e.toString());
    }
}

// Broadcast summary messages to all clients
if (msg.topic.endsWith('/summary')) {
    mqtt.publish('broadcast/summaries', msg.payload, 1, false);
    log.debug('Broadcasted summary from', msg.topic);
}
