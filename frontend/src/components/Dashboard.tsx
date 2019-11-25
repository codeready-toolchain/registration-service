import * as React from 'react';
import { Stack, StackItem } from '@patternfly/react-core';
import MaterialList from './MaterialList';
import DashboardData from './DashboardData';
import TrainingData from './TrainingData';
import MarketingData from './MarketingData';
import '../../public/img/CodeReady_icon_loader.svg';

interface DashboardProps {
  location: {
    state: {
      consoleURL: string;
    };
  };
}

const Dashboard: React.FC<DashboardProps> = (props) => {
  const {
    location: {
      state: { consoleURL },
    },
  } = props;
  DashboardData.materials[0].externalLink = consoleURL;
  return (
    <Stack>
      <StackItem>
        <MaterialList materials={DashboardData.materials} spanValue={6} rowSpanValue={1} />
      </StackItem>
      <StackItem>
        <MaterialList materials={TrainingData.materials} />
      </StackItem>
      <StackItem>
        <MaterialList materials={MarketingData.materials} />
      </StackItem>
    </Stack>
  );
};

export default Dashboard;
