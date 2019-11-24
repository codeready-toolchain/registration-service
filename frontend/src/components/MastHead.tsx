import * as React from 'react';
import { Brand, PageHeader } from '@patternfly/react-core';
import { history } from './utils';
import MastHeadToolbar from './MastHeadToolbar';
import '../../public/img/codeready_toolchain.png';

const MastHead = React.memo(() => {
    const defaultRoute = '/';
    const logoProps = {
      href: defaultRoute,
      // use onClick to prevent browser reload
      onClick: (e) => {
        e.preventDefault();
        history.push(defaultRoute);
      },
    };
    
    return (
      <PageHeader
        logo={<Brand src={'assets/codeready_toolchain.png'} alt={'CodeReady Toolchain'} />}
        logoProps={logoProps}
        toolbar={(window.keycloak && window.keycloak.authenticated)
          ? (<MastHeadToolbar userName={window.keycloak.idTokenParsed.name} />)
          : ("")}
      />
    );
  });

  export default MastHead