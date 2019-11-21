import * as React from 'react';
import { render } from 'react-dom';
import { history } from './utils';
import { Helmet } from 'react-helmet';
import { Route, Router } from 'react-router-dom';
import '@patternfly/react-core/dist/styles/base.css';
import MastHead from './MastHead';
import AppContent from './AppContent';
import AuthLibraryLoader from './AuthLibraryLoader';

//PF4 Imports
import { Page } from '@patternfly/react-core';

// Edge lacks URLSearchParams
import 'url-search-params-polyfill';

const pageId = 'main-content-page-layout-horizontal-nav';
const productName = 'CodeReady ToolChain';
const App: React.FC = () => {
  return (
    <>
      <Helmet titleTemplate={`%s · ${productName}`} defaultTitle={productName} />
      <Page header={<MastHead />} mainContainerId={pageId}>
        <AppContent />
      </Page>
      <AuthLibraryLoader />
    </>
  );
};

render(
  <Router history={history}>
    <Route path="/" component={App} />
  </Router>,
  document.getElementById('app'),
);
