// Rate Limiter
// Prevents message flooding by counting messages per client
// Resets every minute using TTL

const clientId = event.clientId;
const countKey = 'msg_count:' + clientId;

// Get current count (resets every minute via TTL)
const count = state.get(countKey) || 0;

// Check limit
const maxPerMinute = 100;
if (count >= maxPerMinute) {
    log.warn('Rate limit exceeded for client:', clientId,
             '- Message on', event.topic, 'logged but not blocked');
    // Note: Message blocking not implemented yet, just logging
} else {
    // Log every 10 messages
    if ((count + 1) % 10 === 0) {
        log.debug('Client', clientId, 'sent', count + 1, 'messages this minute');
    }
}

// Increment counter with 60 second TTL
state.set(countKey, count + 1, {ttl: 60});
