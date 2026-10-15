import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { App } from './App'
import { ProveedorSesion } from './auth/Sesion'
import './styles.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode><BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}><ProveedorSesion><App /></ProveedorSesion></BrowserRouter></StrictMode>,
)
