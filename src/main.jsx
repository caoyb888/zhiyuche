import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import './index.css'
import Layout    from './components/Layout'
import Dashboard from './pages/Dashboard'
import Approval  from './pages/Approval'
import Trips     from './pages/Trips'
import Reports   from './pages/Reports'
import Health    from './pages/Health'
import Charging  from './pages/Charging'

ReactDOM.createRoot(document.getElementById('root')).render(
  <BrowserRouter>
    <Routes>
      <Route path="/" element={<Layout/>}>
        <Route index          element={<Dashboard/>}/>
        <Route path="approval" element={<Approval/>}/>
        <Route path="trips"    element={<Trips/>}/>
        <Route path="reports"  element={<Reports/>}/>
        <Route path="health"   element={<Health/>}/>
        <Route path="charging" element={<Charging/>}/>
      </Route>
    </Routes>
  </BrowserRouter>
)
