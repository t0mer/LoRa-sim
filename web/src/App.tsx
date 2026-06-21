import { Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import Dashboard from "./pages/Dashboard";
import Tags from "./pages/Tags";
import Traffic from "./pages/Traffic";
import GatewayPage from "./pages/GatewayPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Dashboard />} />
        <Route path="tags" element={<Tags />} />
        <Route path="traffic" element={<Traffic />} />
        <Route path="gateway" element={<GatewayPage />} />
      </Route>
    </Routes>
  );
}
