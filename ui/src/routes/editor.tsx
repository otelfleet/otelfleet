import { createFileRoute } from '@tanstack/react-router'
import {Editor} from '../pages/Editor'

export const Route = createFileRoute('/editor')({
  component: RouteComponent,
})

function RouteComponent() {
  return <Editor></Editor>
}
