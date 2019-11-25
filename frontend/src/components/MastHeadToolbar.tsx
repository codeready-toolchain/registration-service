import * as React from 'react';
import { Toolbar, ToolbarGroup, ToolbarItem, Dropdown, DropdownToggle, DropdownItem } from '@patternfly/react-core';
import accessibleStyles from '@patternfly/react-styles/css/utilities/Accessibility/accessibility';
import { css } from '@patternfly/react-styles';
// import { render } from 'react-dom';

export interface MastHeadToolbarProps {
    userName: string;
    profilePic?: string;
}

const MastHeadToolbar: React.FC<MastHeadToolbarProps> = (props) => {
    const [isDropdownOpen, setDropdownOpenState] = React.useState(false);

    const onDropdownToggle = (isDropdownOpen: boolean) => {
        setDropdownOpenState(isDropdownOpen);
    };

    const onDropdownSelect = (event: any) => {
        setDropdownOpenState(!isDropdownOpen);
    };

    return (
        <Toolbar>
            <ToolbarGroup>
                <ToolbarItem className={css(accessibleStyles.screenReader, accessibleStyles.visibleOnMd)}>
                    <Dropdown
                        isPlain
                        position="right"
                        onSelect={onDropdownSelect}
                        isOpen={isDropdownOpen}
                        toggle={<DropdownToggle onToggle={onDropdownToggle}>{props.userName}</DropdownToggle>}
                        dropdownItems={[
                            <DropdownItem onClick={() => window.keycloak.accountManagement()}>Account</DropdownItem>,
                            <DropdownItem onClick={() => window.keycloak.logout()}>Logout</DropdownItem>,
                        ]}
                    />
                </ToolbarItem>
            </ToolbarGroup>
        </Toolbar>
    );
};

export default MastHeadToolbar;