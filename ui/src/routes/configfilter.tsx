import { createFileRoute } from '@tanstack/react-router'
import { RouterGraphPage } from '../configassignment/RouterGraphPage'

export const Route = createFileRoute('/configfilter')({
  component: RouteComponent,
})

function RouteComponent() {
  return <RouterGraphPage />
}
