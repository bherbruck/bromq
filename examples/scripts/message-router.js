// Message Router
// Transforms and republishes messages to different topics

// Route legacy format to new format
if (event.topic.startsWith('legacy/')) {
    try {
        // Parse legacy format (CSV: sensor_id,value,unit)
        const parts = event.payload.split(',');
        const data = {
            sensor_id: parts[0],
            value: parseFloat(parts[1]),
            unit: parts[2],
            timestamp: Date.now()
        };

        // Republish in new JSON format
        const newTopic = 'v2/' + event.topic.replace('legacy/', '');
        mqtt.publish(newTopic, JSON.stringify(data), event.qos, false);

        log.debug('Transformed message:', event.topic, '->', newTopic);
    } catch (e) {
        log.error('Failed to transform message:', e.toString());
    }
}

// Broadcast summary messages to all clients
if (event.topic.endsWith('/summary')) {
    mqtt.publish('broadcast/summaries', event.payload, 1, false);
    log.debug('Broadcasted summary from', event.topic);
}
