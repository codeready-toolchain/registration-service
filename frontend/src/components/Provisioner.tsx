import * as React from 'react';
import { Stack, StackItem } from '@patternfly/react-core';
import Marketing from './Marketing';
import MarketingData from './MarketingData';
import axios from 'axios';
import { Redirect } from 'react-router-dom';

enum ProvisionStatus {
  PROCESSING,
  PENDING,
  SUCCESS,
  FAILED,
}

const Provisioner: React.FC<{}> = () => {
  const [status, setStatus] = React.useState(ProvisionStatus.PROCESSING);

  const intervalId = React.useRef(null);

  React.useEffect(() => {
    const getUserSignup = () => {
      return axios.get('/api/v1/signup');
    };

    const updateStatus = (status) => {
      console.log('Status :', status);
      if (status['ready']) {
        setStatus(ProvisionStatus.SUCCESS);
        stopStatusPolling();
      } else {
        if (status['reason'] === 'PendingApproval') {
          setStatus(ProvisionStatus.PENDING);
        } else {
          setStatus(ProvisionStatus.PROCESSING);
        }
        intervalId.current = startStatusPolling();
      }
    };

    const startStatusPolling = () => {
      return setInterval(() => {
        getUserSignup()
          .then(({ data }) => {
            updateStatus(data.status);
          })
          .catch(() => {
            console.log('Polling failed');
          });
      }, 60000);
    };

    const stopStatusPolling = () => {
      if (intervalId.current) {
        clearInterval(intervalId.current);
        intervalId.current = null;
      }
    };

    const setUserSignup = () => {
      return axios.post('/api/v1/signup', null);
    };

    window.keycloak.authenticated && getUserSignup()
      .then(({ data }) => {
        updateStatus(data.status);
      })
      .catch(() => {
        setUserSignup();
        intervalId.current = startStatusPolling();
      });

    return () => {
      stopStatusPolling();
    };
  }, []);

  if (!window.keycloak.authenticated) {
    return <Redirect to="/Home" />;
  }

  const icon =
    status === ProvisionStatus.PROCESSING
      ? 'Your CodeReady Toolchain account is being provisioned'
      : status === ProvisionStatus.PENDING
      ? 'Your CodeReady Toolchain account is pending approval'
      : 'Your CodeReady Toolchain account has been provisioned';

  return (
    <Stack>
      <StackItem>
        <div className="provision-section">{icon}</div>
      </StackItem>
      <StackItem>
        <Marketing materials={MarketingData.materials} />
      </StackItem>
    </Stack>
  );
};

export default Provisioner;
