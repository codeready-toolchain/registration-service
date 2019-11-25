import * as React from 'react';
import { render } from 'react-dom';
import { history } from './utils';
import { Helmet } from 'react-helmet';
import { Route, Router } from 'react-router-dom';
import '@patternfly/react-core/dist/styles/base.css';
import MastHead from './MastHead';
import AppContent from './AppContent';
import AuthLibraryLoader from './AuthLibraryLoader';
import './App.scss';

//PF4 Imports
import { Page } from '@patternfly/react-core';

// Edge lacks URLSearchParams
import 'url-search-params-polyfill';

const pageId = 'main-content-page-layout-horizontal-nav';
const productName = 'CodeReady ToolChain';
const App: React.FC = () => {
  const [userInfo, setUserInfo] = React.useState(null);

  const fetchUserInfo = React.useCallback(() => {
    window.keycloak.loadUserInfo().success((data)=>{
      setUserInfo(data);
    });
  }, [window.keycloak]);

  React.useEffect(() => {
    window.keycloak && fetchUserInfo();
  }, [window.keycloak]);

  return (
    <>
      <AuthLibraryLoader />
      <Helmet titleTemplate={`%s · ${productName}`} defaultTitle={productName} />
      <Page header={<MastHead userInfo={userInfo} />} mainContainerId={pageId}>
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
