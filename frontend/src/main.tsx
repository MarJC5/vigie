import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

// The application shell comes with the design system components; this entry point only mounts it.
function App() {
  return null;
}

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}
