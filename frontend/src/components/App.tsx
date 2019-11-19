import * as React from 'react';
import { render } from 'react-dom';
import { history } from './utils';
import MastHead from './MastHead';
import { Helmet } from 'react-helmet';
import { Route, Router } from 'react-router-dom';
import AppContent from './AppContent';
import '@patternfly/react-core/dist/styles/base.css';

//PF4 Imports
import { Page } from '@patternfly/react-core';

// Edge lacks URLSearchParams
import 'url-search-params-polyfill';

const productName = 'CodeReady ToolChain';
const App: React.FC = () => {
  return (
    <>
      <Helmet titleTemplate={`%s · ${productName}`} defaultTitle={productName} />
      <Page header={<MastHead />}>
        <AppContent />
      </Page>
    </>
  );
};

render(
  <Router history={history}>
    <Route path="/" component={App} />
  </Router>,
  document.getElementById('app'),
);
