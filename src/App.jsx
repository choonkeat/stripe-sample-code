import React from "react";
import Home from "./Home";
import Refresh from "./Refresh";
import Return from "./Return";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import "./App.css";

const router = createBrowserRouter([
  {
    path: "/",
    element: <Home />,
  },
  {
    path: "/refresh/:connectedAccountId",
    element: <Refresh />,
  },
  {
    path: "/return/:connectedAccountId",
    element: <Return />,
  },
]);

export default function App() {
  return (
    <div>
      <RouterProvider router={router} />
    </div>
  );
}