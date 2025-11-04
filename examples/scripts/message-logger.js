// Message Logger
// Logs all published messages with metadata

log.info('Message received:', msg.topic);
log.debug('Payload:', msg.payload);
log.debug('Client:', msg.clientId, 'User:', msg.username);
log.debug('QoS:', msg.qos, 'Retain:', msg.retain);
