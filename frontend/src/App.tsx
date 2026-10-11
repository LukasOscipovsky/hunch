import { useEffect, useState } from 'react'
import './App.css'

function App() {
  const [message, setMessage] = useState('Loading...')
  const [standings, setStandings] = useState('Loading...')
  const [rosters, setRosters] = useState('Loading...')

  useEffect(() => {
    fetch('/api/hello')
      .then((res) => res.json())
      .then((data: { message: string }) => setMessage(data.message))
      .catch(() => setMessage('Failed to reach backend'))

    fetch('/api/standings')
      .then((res) => {
        if (!res.ok) throw new Error(res.statusText)
        return res.json()
      })
      .then((data: unknown) => setStandings(JSON.stringify(data, null, 2)))
      .catch(() => setStandings('Failed to load NHL standings'))

    fetch('/api/rosters')
      .then((res) => {
        if (!res.ok) throw new Error(res.statusText)
        return res.json()
      })
      .then((data: unknown) => setRosters(JSON.stringify(data, null, 2)))
      .catch(() => setRosters('Failed to load NHL rosters'))
  }, [])

  return (
    <>
      <h1>Hello World</h1>
      <p>{message}</p>
      <h2>NHL standings</h2>
      <pre>{standings}</pre>
      <h2>NHL rosters</h2>
      <pre>{rosters}</pre>
    </>
  )
}

export default App
