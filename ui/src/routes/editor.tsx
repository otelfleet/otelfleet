import { createFileRoute } from '@tanstack/react-router'
import { Editor } from '../pages/Editor'

type EditorSearch = {
  id?: string
}

export const Route = createFileRoute('/editor')({
  validateSearch: (search: Record<string, unknown>): EditorSearch => {
    return {
      id: typeof search.id === 'string' ? search.id : undefined,
    }
  },
  component: RouteComponent,
})

function RouteComponent() {
  const { id } = Route.useSearch()
  return <Editor configId={id} />
}
