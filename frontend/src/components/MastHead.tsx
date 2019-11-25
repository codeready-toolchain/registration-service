import * as React from 'react';
import { Brand, PageHeader } from '@patternfly/react-core';
import { history } from './utils';
import MastHeadToolbar from './MastHeadToolbar';
import '../../public/img/codeready_toolchain.png';

interface MastHeadProps {
  userInfo: {
    email: string;
    family_name: string;
    given_name: string;
    name: string;
    preferred_username: string;
  };
}

const MastHead = React.memo((props: MastHeadProps) => {
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
      toolbar={
        props.userInfo ? (
          <MastHeadToolbar userName={props.userInfo.name} />
        ) : (
          ''
        )
      }
    />
  );
});

export default MastHead;
