// Message Logger
// Logs all published messages with metadata

log.info('Message received:', event.topic);
log.debug('Payload:', event.payload);
log.debug('Client:', event.clientId, 'User:', event.username);
log.debug('QoS:', event.qos, 'Retain:', event.retain);
