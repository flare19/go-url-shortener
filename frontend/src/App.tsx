import { BrowserRouter, Routes, Route } from "react-router-dom";
import Nav from "./components/Nav";
import ShortenPage from "./pages/ShortenPage";
import StatsPage from "./pages/StatsPage";

export default function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen bg-[#0a0b0d]">
        <Nav />
        <Routes>
          <Route path="/" element={<ShortenPage />} />
          <Route path="/stats" element={<StatsPage />} />
        </Routes>
      </div>
    </BrowserRouter>
  );
}