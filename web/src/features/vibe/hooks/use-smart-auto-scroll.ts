import {
  type RefObject,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

const NEAR_BOTTOM_THRESHOLD_PX = 80;

function isNearBottom(element: HTMLElement) {
  return (
    element.scrollHeight - element.clientHeight - element.scrollTop <=
    NEAR_BOTTOM_THRESHOLD_PX
  );
}

export function useSmartAutoScroll({
  contentKey,
  resetKey,
}: {
  contentKey: string;
  resetKey: string;
}) {
  const scrollContainerRef = useRef<HTMLDivElement | null>(null);
  const bottomAnchorRef = useRef<HTMLDivElement | null>(null);
  const contentKeyRef = useRef("");
  const isFollowingRef = useRef(true);
  const [jumpToLatestState, setJumpToLatestState] = useState({
    resetKey,
    show: false,
  });
  const showJumpToLatest =
    jumpToLatestState.resetKey === resetKey && jumpToLatestState.show;

  const scrollToLatest = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      const container = scrollContainerRef.current;
      if (container === null) {
        return;
      }

      bottomAnchorRef.current?.scrollIntoView({
        behavior,
        block: "end",
      });
      container.scrollTop = container.scrollHeight;
      isFollowingRef.current = true;
      setJumpToLatestState({ resetKey, show: false });
    },
    [resetKey],
  );

  const handleScroll = useCallback(() => {
    const container = scrollContainerRef.current;
    if (container === null) {
      return;
    }

    const following = isNearBottom(container);
    isFollowingRef.current = following;

    if (following) {
      setJumpToLatestState({ resetKey, show: false });
    }
  }, [resetKey]);

  useEffect(() => {
    contentKeyRef.current = "";
    isFollowingRef.current = true;
  }, [resetKey]);

  useLayoutEffect(() => {
    if (contentKey.length === 0) {
      return;
    }

    const container = scrollContainerRef.current;
    if (container === null) {
      return;
    }

    if (contentKeyRef.current.length === 0) {
      contentKeyRef.current = contentKey;
      scrollToLatest("auto");
      return;
    }

    if (contentKeyRef.current === contentKey) {
      return;
    }

    contentKeyRef.current = contentKey;

    const currentlyNearBottom = isNearBottom(container);

    if (isFollowingRef.current && currentlyNearBottom) {
      scrollToLatest("smooth");
      return;
    }

    if (!currentlyNearBottom) {
      isFollowingRef.current = false;
      setJumpToLatestState({ resetKey, show: true });
    }
  }, [contentKey, resetKey, scrollToLatest]);

  return {
    bottomAnchorRef,
    handleScroll,
    scrollContainerRef,
    scrollToLatest,
    showJumpToLatest,
  } satisfies {
    bottomAnchorRef: RefObject<HTMLDivElement | null>;
    handleScroll: () => void;
    scrollContainerRef: RefObject<HTMLDivElement | null>;
    scrollToLatest: (behavior?: ScrollBehavior) => void;
    showJumpToLatest: boolean;
  };
}
