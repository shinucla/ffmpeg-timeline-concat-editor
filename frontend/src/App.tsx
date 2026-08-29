import { AppNav } from './components/AppNav'
import { Library } from './components/Library'
import { ProcessPanel } from './components/ProcessPanel'
import { TrimEditor } from './components/TrimEditor'
import './App.css'

function App() {
  return (
    <div className="app-shell">
      <AppNav />

      <div className="app-scroll">
        <div className="app">
          <main className="layout">
            <aside className="sidebar">
              <Library />
            </aside>
            <div className="main">
              <TrimEditor />
            </div>
          </main>
        </div>
      </div>

      <ProcessPanel />
    </div>
  )
}

export default App
