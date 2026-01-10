import { createFileRoute } from '@tanstack/react-router'
import {AgentPage} from '../pages/ListAgentPage'

export const Route = createFileRoute('/agents')({
  component: RouteComponent,
})

function RouteComponent() {
  return <AgentPage></AgentPage>
}
