import { createFileRoute } from '@tanstack/react-router'
import {TokenPage} from '../pages/ListTokenPage'

export const Route = createFileRoute('/tokens')({
  component: RouteComponent,
})

function RouteComponent() {
  return <TokenPage></TokenPage>
}
