import { Routes, Route } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { Layout } from '@/components/Layout'
import { LoginPage } from '@/pages/LoginPage'
import { DashboardPage } from '@/pages/DashboardPage'
import { PortfolioPage } from '@/pages/PortfolioPage'
import { MarketDataPage } from '@/pages/MarketDataPage'
import { NewsPage } from '@/pages/NewsPage'
import { ScreenerPage } from '@/pages/ScreenerPage'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import './App.css'

function App() {
  const { isAuthenticated } = useAuthStore()

  return (
    <div className="min-h-screen bg-gray-900 text-white">
      <Routes>
        <Route path="/login" element={<LoginPage />} />

        <Route
          path="/*"
          element={
            <ProtectedRoute isAuthenticated={isAuthenticated}>
              <Layout>
                <Routes>
                  <Route path="/" element={<DashboardPage />} />
                  <Route path="/portfolio" element={<PortfolioPage />} />
                  <Route path="/market" element={<MarketDataPage />} />
                  <Route path="/news" element={<NewsPage />} />
                  <Route path="/screener" element={<ScreenerPage />} />
                </Routes>
              </Layout>
            </ProtectedRoute>
          }
        />
      </Routes>
    </div>
  )
}

export default App