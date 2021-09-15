import React, { FC, memo } from 'react';
import { css } from '@emotion/css';
import { styleMixins, useTheme2, CustomScrollbar, IconButton } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';
import { contextSrv } from 'app/core/services/context_srv';
import { useSearchQuery } from '../hooks/useSearchQuery';
import { useDashboardSearch } from '../hooks/useDashboardSearch';
import { SearchField } from './SearchField';
import { SearchResults } from './SearchResults';
import { ActionRow } from './ActionRow';

export interface Props {
  onCloseSearch: () => void;
}

export const DashboardSearch: FC<Props> = memo(({ onCloseSearch }) => {
  const navPosition = contextSrv.user.navPosition;
  const { query, onQueryChange, onTagFilterChange, onTagAdd, onSortChange, onLayoutChange } = useSearchQuery({});
  const { results, loading, onToggleSection, onKeyDown } = useDashboardSearch(query, onCloseSearch);
  const theme = useTheme2();
  const styles = getStyles(theme, navPosition);

  return (
    <div tabIndex={0} className={styles.overlay}>
      <div className={styles.container}>
        <div className={styles.searchField}>
          <SearchField query={query} onChange={onQueryChange} onKeyDown={onKeyDown} autoFocus clearable />
          <div className={styles.closeBtn}>
            <IconButton name="times" surface="panel" onClick={onCloseSearch} size="xxl" tooltip="Close search" />
          </div>
        </div>
        <div className={styles.search}>
          <ActionRow
            {...{
              onLayoutChange,
              onSortChange,
              onTagFilterChange,
              query,
            }}
          />
          <CustomScrollbar>
            <SearchResults
              results={results}
              loading={loading}
              onTagSelected={onTagAdd}
              editable={false}
              onToggleSection={onToggleSection}
              layout={query.layout}
            />
          </CustomScrollbar>
        </div>
      </div>
    </div>
  );
});

DashboardSearch.displayName = 'DashboardSearch';

export default DashboardSearch;

const getStyles = (theme: GrafanaTheme2, navPosition: typeof contextSrv.user.navPosition) => ({
  overlay: css`
    left: 0;
    top: 0;
    right: 0;
    bottom: 0;
    z-index: ${theme.zIndex.sidemenu};
    position: fixed;
    background: ${theme.colors.background.canvas};

    @media ${styleMixins.mediaUp(`${theme.breakpoints.values.md}px`)} {
      bottom: ${navPosition === 'bottom' ? `${theme.components.sidemenu.width}px` : 0};
      left: ${navPosition === 'left' ? `${theme.components.sidemenu.width}px` : 0};
      right: ${navPosition === 'right' ? `${theme.components.sidemenu.width}px` : 0};
      top: ${navPosition === 'top' ? `${theme.components.sidemenu.width}px` : 0};
      z-index: ${theme.zIndex.navbarFixed + 1};
    }
  `,
  container: css`
    max-width: 1400px;
    margin: 0 auto;
    padding: ${theme.spacing(4)};

    height: 100%;

    @media ${styleMixins.mediaUp(`${theme.breakpoints.values.md}px`)} {
      padding: 32px;
    }
  `,
  closeBtn: css`
    right: -5px;
    top: 2px;
    z-index: 1;
    position: absolute;
  `,
  searchField: css`
    position: relative;
  `,
  search: css`
    display: flex;
    flex-direction: column;
    height: 100%;
  `,
});
