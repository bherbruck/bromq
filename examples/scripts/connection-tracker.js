// Connection Tracker
// Tracks client connection durations using state
// Use with on_connect and on_disconnect triggers

if (event.type === 'connect') {
    // Store connection time
    state.set('connect_time:' + event.clientId, Date.now());

    log.info('Client connected:', event.clientId, 'User:', event.username);

    // Track total connections in global state
    const totalConnects = global.get('total_connections') || 0;
    global.set('total_connections', totalConnects + 1);

} else if (event.type === 'disconnect') {
    // Calculate session duration
    const connectTime = state.get('connect_time:' + event.clientId);

    if (connectTime) {
        const duration = Date.now() - connectTime;
        const durationSeconds = Math.floor(duration / 1000);

        log.info('Client disconnected:', event.clientId,
                 'Duration:', durationSeconds, 'seconds');

        // Store duration in history
        const historyKey = 'duration_history:' + event.clientId;
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
        state.delete('connect_time:' + event.clientId);
    }
}
