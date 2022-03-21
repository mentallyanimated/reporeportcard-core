import React from 'react';
import { Routes, Route } from "react-router-dom";

const HomePage = React.lazy(() => import('./pages/HomePage'));
const GraphPage = React.lazy(() => import('./pages/GraphPage'));

const App = () => {
  return (
    <React.Suspense fallback={<div>loading...</div>}>
      <Routes>
        <Route exact path="/" element={<HomePage />} />
        <Route exact path="/graph" element={<GraphPage/>} />
      </Routes>
    </React.Suspense>
  );
}

export default App;
