// Connection Tracker
// Tracks client connection durations using state
// Use with on_connect and on_disconnect triggers

if (msg.type === 'connect') {
    // Store connection time
    state.set('connect_time:' + msg.clientId, Date.now());

    log.info('Client connected:', msg.clientId, 'User:', msg.username);

    // Track total connections in global state
    const totalConnects = global.get('total_connections') || 0;
    global.set('total_connections', totalConnects + 1);

} else if (msg.type === 'disconnect') {
    // Calculate session duration
    const connectTime = state.get('connect_time:' + msg.clientId);

    if (connectTime) {
        const duration = Date.now() - connectTime;
        const durationSeconds = Math.floor(duration / 1000);

        log.info('Client disconnected:', msg.clientId,
                 'Duration:', durationSeconds, 'seconds');

        // Store duration in history
        const historyKey = 'duration_history:' + msg.clientId;
        const history = state.get(historyKey) || [];
        history.push({
            connected_at: connectTime,
            disconnected_at: Date.now(),
            duration: duration
        });

        // Keep last 20 sessions
        if (history.length > 20) {
            history.shift();
        }

        state.set(historyKey, history);
        state.delete('connect_time:' + msg.clientId);
    }
}
