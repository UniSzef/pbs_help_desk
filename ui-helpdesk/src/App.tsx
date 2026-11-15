import './App.css'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

function App() {
  return (
    <main className="app">
      <Card className="w-full max-w-md">
        <CardHeader>
          <Badge className="w-fit" variant="accent">
            Frontend
          </Badge>
          <CardTitle className="text-3xl">pbs-helpdesk</CardTitle>
          <CardDescription>Panel startowy aplikacji</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">Aplikacja jest gotowa do dalszej rozbudowy.</p>
        </CardContent>
      </Card>
    </main>
  )
}

export default App
