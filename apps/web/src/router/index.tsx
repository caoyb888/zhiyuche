import { Suspense, lazy } from 'react'
import { createBrowserRouter, RouterProvider, type RouteObject } from 'react-router-dom'
import Layout from '../components/Layout'
import PageLoading from '../components/PageLoading'
import Forbidden from '../pages/Forbidden'
import NotFound from '../pages/NotFound'
import { PermissionGuard, PublicOnly, RequireAuth } from './guards'
import { appRoutes } from './routes'

const Login = lazy(() => import('../pages/auth/Login'))

const protectedChildren: RouteObject[] = appRoutes.map((r) => {
  const Page = r.component
  const element = (
    <PermissionGuard perm={r.perm}>
      <Suspense fallback={<PageLoading />}>
        <Page />
      </Suspense>
    </PermissionGuard>
  )
  return r.path === '/' ? { index: true, element } : { path: r.path, element }
})

const router = createBrowserRouter([
  {
    path: '/login',
    element: (
      <PublicOnly>
        <Suspense fallback={<PageLoading />}>
          <Login />
        </Suspense>
      </PublicOnly>
    ),
  },
  {
    element: <RequireAuth />,
    children: [
      {
        path: '/',
        element: <Layout />,
        children: [...protectedChildren, { path: '/403', element: <Forbidden /> }, { path: '*', element: <NotFound /> }],
      },
    ],
  },
])

export default function AppRouter() {
  return <RouterProvider router={router} />
}
