import * as React from 'react';
import { Stack, StackItem } from '@patternfly/react-core';
import MaterialList from './MaterialList';
import MarketingData from './MarketingData';
import axios from 'axios';
import { Redirect } from 'react-router-dom';
import { Spinner } from '@patternfly/react-core/dist/esm/experimental';
import { OkIcon } from '@patternfly/react-icons';

enum ProvisionStatus {
  UNPROVISIONED,
  PROCESSING,
  PENDING,
  SUCCESS,
  FAILED,
  DASHBOARD,
}

export interface ProvisionerMessage {
  [name: string]: string;
}

const provisionerMsg = {
  [ProvisionStatus.PROCESSING]: 'Your CodeReady Toolchain account is being provisioned',
  [ProvisionStatus.PENDING]: 'Your CodeReady Toolchain account approval is pending',
  [ProvisionStatus.SUCCESS]: 'Your CodeReady Toolchain account has been provisioned',
  [ProvisionStatus.FAILED]: 'Internal server error. Please try again later',
} as ProvisionerMessage;

const Provisioner: React.FC<{}> = () => {
  const [status, setStatus] = React.useState(ProvisionStatus.UNPROVISIONED);

  const intervalId = React.useRef(null);
  const consoleURLRef = React.useRef(null);

  React.useEffect(() => {
    const getUserSignup = () => {
      return axios.get('/api/v1/signup');
    };

    const updateStatus = ({ status, consoleURL }) => {
      if (status.ready) {
        consoleURLRef.current = consoleURL;
        setStatus(ProvisionStatus.SUCCESS);
        stopStatusPolling();
      } else {
        if (status.reason === 'PendingApproval') {
          setStatus(ProvisionStatus.PENDING);
        } else {
          setStatus(ProvisionStatus.PROCESSING);
        }
      }
    };

    const startStatusPolling = () => {
      return setInterval(() => {
        getUserSignup()
          .then(({ data }) => {
            updateStatus(data);
          })
          .catch((error) => {
            setStatus(ProvisionStatus.FAILED);
          });
      }, 15000);
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

    window.keycloak.authenticated &&
      getUserSignup()
        .then(({ data }) => {
          updateStatus(data);
          if (!data.status.ready) {
            intervalId.current = startStatusPolling();
          }
        })
        .catch((error) => {
          if (error.response && error.response.status && error.response.status === 404) {
            setUserSignup()
              .then((response) => {
                setStatus(ProvisionStatus.PROCESSING);
                intervalId.current = startStatusPolling();
              })
              .catch((error) => {
                setStatus(ProvisionStatus.FAILED);
              });
          } else {
            setStatus(ProvisionStatus.FAILED);
          }
        });

    return () => {
      stopStatusPolling();
    };
  }, []);

  const formatStatusMessage = (): React.ReactElement => {
    switch (status) {
      case ProvisionStatus.PROCESSING:
      case ProvisionStatus.PENDING:
        return (
          <>
            {' '}
            <Spinner size="lg" /> {provisionerMsg[status]}{' '}
          </>
        );
      case ProvisionStatus.SUCCESS:
        return (
          <>
            {' '}
            <OkIcon /> {provisionerMsg[status]}{' '}
          </>
        );
      case ProvisionStatus.FAILED:
        return <> {provisionerMsg[status]} </>;
      default:
        return null;
    }
  };

  if (!window.keycloak.authenticated) {
    return <Redirect to="/Home" />;
  }

  if (status === ProvisionStatus.DASHBOARD) {
    return <Redirect to={{ pathname: '/Dashboard', state: { consoleURL: consoleURLRef.current} }} />;
  } else {
    if (status === ProvisionStatus.SUCCESS) {
      setTimeout(() => {
        setStatus(ProvisionStatus.DASHBOARD);
      }, 5000);
    }
    const statusData = formatStatusMessage();
    return (
      <Stack>
        <StackItem>
          <div className="provision-section">
            <span>{statusData}</span>
          </div>
        </StackItem>
        <StackItem>
          <MaterialList materials={MarketingData.materials} />
        </StackItem>
      </Stack>
    );
  }
};

export default Provisioner;
