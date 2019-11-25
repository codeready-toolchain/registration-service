import * as React from 'react';
import { Stack, StackItem } from '@patternfly/react-core';
import MaterialList from './MaterialList';
import MarketingData from './MarketingData';
import axios from 'axios';
import { Redirect } from 'react-router-dom';
import { ErrorCircleOIcon, PendingIcon, CheckCircleIcon, Spinner2Icon } from '@patternfly/react-icons';

enum ProvisionStatus {
  PROCESSING,
  PENDING,
  SUCCESS,
  FAILED,
  DASHBOARD,
}

export interface ProvisionerMessage {
  [name: string]: string;
}

type ProvisionerStatusData = {
    icon?: React.ReactElement;
    text?: string
}

const provisionerMsg = {
  [ProvisionStatus.PROCESSING]: 'Your CodeReady Toolchain account is being provisioned',
  [ProvisionStatus.PENDING]: 'Your CodeReady Toolchain account approval is pending',
  [ProvisionStatus.SUCCESS]: 'Your CodeReady Toolchain account has been provisioned',
  [ProvisionStatus.FAILED]:
    'Your CodeReady Toolchain account provisioning has failed. Please contact administrator',
} as ProvisionerMessage;

const Provisioner: React.FC<{}> = () => {
  const [status, setStatus] = React.useState(ProvisionStatus.PROCESSING);

  const intervalId = React.useRef(null);

  React.useEffect(() => {
    const getUserSignup = () => {
      return axios.get('/api/v1/signup');
    };

    const updateStatus = (status) => {
      console.log('Status :', status);
      if (status.ready) {
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
            updateStatus(data.status);
          })
          .catch((status) => {
            console.log('Polling failed', status);
            setStatus(ProvisionStatus.FAILED);
          });
      }, 30000);
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
          updateStatus(data.status);
          if (!data.status.ready) {
            intervalId.current = startStatusPolling();
          }
        })
        .catch(() => {
          /* TODO: Need to check state in redirect URL before provisioning */
          setUserSignup();
          intervalId.current = startStatusPolling();
        });

    return () => {
      stopStatusPolling();
    };
  }, []);

  const formatStatusMessage = ():ProvisionerStatusData => {
    switch (status) {
        case ProvisionStatus.PROCESSING:
            return {icon: <Spinner2Icon color="yellow" size="md" />, text: provisionerMsg[status]};
        case ProvisionStatus.PENDING:
            return {icon: <PendingIcon color="yellow" size="md" />, text: provisionerMsg[status]};
        case ProvisionStatus.SUCCESS:
            return {icon: <CheckCircleIcon color="green" size="md" />, text: provisionerMsg[status]};
        case ProvisionStatus.FAILED:
            return {icon: <ErrorCircleOIcon color="red" size="md" />, text: provisionerMsg[status]};

        default:
            return {};
    }
};

  if (!window.keycloak.authenticated) {
    return <Redirect to="/Home" />;
  }

  if (status === ProvisionStatus.DASHBOARD) {
    return <Redirect to="/Dashboard" />;
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
              {statusData.icon}
              {statusData.text}
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
