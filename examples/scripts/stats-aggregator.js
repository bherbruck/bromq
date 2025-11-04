// Statistics Aggregator
// Aggregates message statistics across all topics using global state

// Update message counters
const totalKey = 'total_messages';
const total = global.get(totalKey) || 0;
global.set(totalKey, total + 1);

// Count by topic prefix
const prefix = msg.topic.split('/')[0];
const prefixKey = 'topic_count:' + prefix;
const prefixCount = global.get(prefixKey) || 0;
global.set(prefixKey, prefixCount + 1);

// Count by client
const clientKey = 'client_count:' + msg.clientId;
const clientCount = global.get(clientKey) || 0;
global.set(clientKey, clientCount + 1);

// Every 1000 messages, publish stats
if ((total + 1) % 1000 === 0) {
    log.info('Milestone reached:', total + 1, 'total messages processed');

    mqtt.publish('stats/totals', JSON.stringify({
        total_messages: total + 1,
        timestamp: Date.now()
    }), 1, false);
}
