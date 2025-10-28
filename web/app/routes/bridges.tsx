import type { Route } from './+types/bridges'
import { BridgeList } from '~/components/bridge-list'

export const meta: Route.MetaFunction = () => [{ title: 'Bridges - MQTT Server' }]

export default function BridgesPage() {
  return <BridgeList />
}
