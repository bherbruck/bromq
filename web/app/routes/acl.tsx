import type { Route } from './+types/acl'
import { ACLRules } from '~/components/acl-rules'

export const meta: Route.MetaFunction = () => [{ title: 'ACL Rules - BroMQ' }]

export default function ACLPage() {
  return <ACLRules showMQTTUserColumn={true} showHeader={true} />
}
